"""Prepare local WAV/MP3 references, export Pocket voices, and audition them offline."""
import argparse
import hashlib
import json
import math
from pathlib import Path
import re
import time

import numpy as np
import soundfile as sf
from scipy.io import wavfile
from scipy.signal import resample_poly

from runtime import LANGUAGE, ROOT, RUNTIME, load_model, synthesis_options, synthesis_settings, voice_source

SAMPLE_TEXT = (
    "Welcome, traveler. The road to the village is dangerous after sunset. "
    "Speak with the captain at the gates, and tell her that help is on the way."
)
SAMPLE_DECODE_STEPS = (1, 2, 4)
SAMPLES_DIR = RUNTIME.parent / "voice-samples"
PROMPT_LIMIT_SECONDS = 30


def read_audio(path):
    path = Path(path)
    if path.suffix.lower() not in (".wav", ".mp3"):
        raise ValueError(f"Unsupported reference format: {path.suffix}; use WAV or MP3")
    try:
        audio, rate = sf.read(path, dtype="float64")
    except (sf.SoundFileError, OSError) as exc:
        raise ValueError(f"Cannot decode audio reference {path}: {exc}") from exc
    if audio.ndim not in (1, 2) or not audio.size or not np.isfinite(audio).all():
        raise ValueError("Audio must contain finite mono or stereo samples")
    if rate <= 0:
        raise ValueError("Invalid audio sample rate")
    clipped = float(np.mean(np.abs(audio) >= 0.999))
    if audio.ndim == 2:
        mono = audio.mean(axis=1)
        # Avoid cancelling a stereo recording with opposite channel polarity.
        if np.mean(mono ** 2) < 0.1 * np.mean(audio ** 2):
            mono = audio[:, np.argmax(np.mean(audio ** 2, axis=0))]
        audio = mono
    if np.max(np.abs(audio)) < 1e-5:
        raise ValueError("Reference is silent")
    return rate, audio, clipped


def excerpt(audio, rate, seconds, start=None):
    """Use recordings under 30s whole; otherwise select 0s through the first quiet
    pause at/after `seconds`. --start uses a fixed window instead."""
    if start is not None:
        first = round(start * rate)
        if first < 0 or first >= len(audio):
            raise ValueError("Start time is outside the recording")
        last = min(first + min(len(audio), round(seconds * rate)), len(audio))
    elif len(audio) < PROMPT_LIMIT_SECONDS * rate:
        first, last = 0, len(audio)
    else:
        # Voice lines should run from the start of the recording and end between
        # lines: find the first pause after the minimum duration. A pause must
        # survive 0.2s so stop-closure gaps do not end the excerpt mid-sentence.
        first = 0
        hop = max(1, round(rate * 0.1))
        rms = np.array([np.sqrt(np.mean(audio[i:i+hop] ** 2))
                        for i in range(0, len(audio), hop)])
        threshold = max(0.003, float(np.percentile(rms, 90)) * 0.12)
        quiet = np.convolve((rms < threshold).astype(float), np.ones(2), "valid")
        lo, hi = round(seconds * rate), min(len(audio), PROMPT_LIMIT_SECONDS * rate)
        last = hi
        if lo < hi:
            begin = -(-lo // hop)  # first hop starting at/after the minimum duration
            pauses = (i for i in range(begin, min(len(quiet), hi // hop)) if quiet[i] >= 2)
            pause = next(pauses, None)
            if pause is not None:
                last = pause * hop
            else:
                window = rms[begin : hi // hop]
                if window.size:
                    # No pause in range; end at the quietest hop instead.
                    last = (begin + int(np.argmin(window))) * hop
    if last - first < rate * 3:
        raise ValueError("Reference needs at least three seconds of audio")
    clip = audio[first:last].copy()
    # Gentle peak adjustment and very short edge fades; do not remove room tone.
    peak = np.max(np.abs(clip))
    if peak < 1e-5:
        raise ValueError("Selected excerpt is silent; choose another start time")
    clip *= min(4.0, 0.9 / peak)
    fade = min(round(rate * 0.005), len(clip)//2)
    if fade:
        clip[:fade] *= np.linspace(0, 1, fade)
        clip[-fade:] *= np.linspace(1, 0, fade)
    divisor = math.gcd(rate, 24000)
    clip = resample_poly(clip, 24000 // divisor, rate // divisor)
    return clip, first / rate, last / rate


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--voice", action="append", required=True, metavar="PROFILE=AUDIO",
                        help="Configured profile and local WAV or MP3 file; repeat for multiple voices")
    parser.add_argument("--config", type=Path, default=ROOT / "tts/voices.json")
    parser.add_argument("--seconds", type=float, default=20,
                        help="Minimum reference duration; recordings under 30s are used whole, longer ones end at the first quiet pause at/after this")
    parser.add_argument("--start", type=float, help="Explicit excerpt start for a fixed --seconds window")
    parser.add_argument("--prepare-only", action="store_true", help="Prepare WAVs without loading a model")
    parser.add_argument("--online", action="store_true", help="Allow downloading cloning weights; audio stays local")
    parser.add_argument("--activate", action="store_true", help="Assign profiles after every export and sample succeeds")
    parser.add_argument("--force", action="store_true", help="Replace existing exported voices")
    parser.add_argument("--decode-steps", type=int, help="1-64 decoding steps for previews and activated profiles; more work is not guaranteed better")
    parser.add_argument("--cpu-threads", type=int, help="CPU threads for Python previews only; app uses -cpu-threads")
    args = parser.parse_args()
    if not math.isfinite(args.seconds) or not 3 <= args.seconds <= 30:
        parser.error("--seconds must be between 3 and 30")
    if args.start is not None and (not math.isfinite(args.start) or args.start < 0):
        parser.error("--start must be a finite nonnegative number")
    if args.prepare_only and args.activate:
        parser.error("--activate requires export; remove --prepare-only")
    original = args.config.read_bytes()
    config = json.loads(original)
    try:
        for profile in config["profiles"].values():
            voice_source(args.config, profile)
    except ValueError as exc:
        parser.error(str(exc))
    output = args.config.resolve().parent / "custom"
    jobs, names = [], set()
    for item in args.voice:
        name, separator, source = item.partition("=")
        if not separator or not re.fullmatch(r"[a-zA-Z0-9_-]+", name) or name in names:
            parser.error("Use unique PROFILE=AUDIO pairs (WAV or MP3) with simple profile names")
        if name not in config["profiles"]:
            parser.error(f"Unknown configured profile: {name}")
        for key in ("decode_steps", "cpu_threads"):
            value = getattr(args, key)
            if value is not None:
                config["profiles"][name][key] = value
        try:
            synthesis_options(config["profiles"][name])
        except ValueError as exc:
            parser.error(str(exc))
        if not args.prepare_only and (output / f"{name}.safetensors").exists() and not args.force:
            parser.error(f"{name} already exists; use --force to replace it")
        names.add(name)
        rate, audio, clipped = read_audio(Path(source))
        clip, first, last = excerpt(audio, rate, args.seconds, args.start)
        jobs.append((name, clip, {
            "source": str(Path(source).resolve()),
            "source_sha256": hashlib.sha256(Path(source).read_bytes()).hexdigest(),
            "source_seconds": len(audio)/rate, "source_clipped_fraction": clipped,
            "start_seconds": first, "end_seconds": last,
            "sample_rate": 24000, "language": LANGUAGE,
            "selection": ("manual" if args.start is not None
                          else "whole recording (under 30s)" if len(audio) < PROMPT_LIMIT_SECONDS * rate
                          else "0s through first quiet pause at/after minimum duration"),
        }))
    output.mkdir(parents=True, exist_ok=True)
    for name, clip, metadata in jobs:
        wavfile.write(output / f"{name}.reference.wav", 24000,
                      (clip.clip(-1, 1) * 32767).astype(np.int16))
        (output / f"{name}.reference.json").write_text(json.dumps(metadata, indent=2) + "\n")
        print(f"{name}: selected {metadata['start_seconds']:.1f}-{metadata['end_seconds']:.1f}s "
              f"of {metadata['source_seconds']:.1f}s; source clipping {metadata['source_clipped_fraction']:.2%}", flush=True)
    if args.prepare_only:
        return
    print("Loading cloning model on CPU...", flush=True)
    model = load_model(offline=not args.online)
    if not model.has_voice_cloning:
        raise SystemExit("Cloning weights unavailable. Accept access at https://huggingface.co/kyutai/pocket-tts, "
                         "run `uv run --project tools/voices hf auth login` from the repository root, "
                         "and rerun with --online. Profiles were not changed.")
    from pocket_tts import export_model_state
    SAMPLES_DIR.mkdir(parents=True, exist_ok=True)

    def preview(restored, profile, path):
        with synthesis_settings(model, profile):
            generated = model.generate_audio(restored, SAMPLE_TEXT, copy_state=True).detach().cpu().numpy()
        if not generated.size or not np.isfinite(generated).all() or np.max(np.abs(generated)) < 1e-5:
            raise RuntimeError(f"Invalid generated sample {path.name}; profiles were not changed")
        wavfile.write(path, model.sample_rate, (generated.clip(-1, 1) * 32767).astype(np.int16))
        return generated

    for name, _, metadata in jobs:
        started = time.monotonic()
        state = model.get_state_for_audio_prompt(output / f"{name}.reference.wav", truncate=True)
        temporary = output / f"{name}.pending.safetensors"
        export_model_state(state, str(temporary))
        # Verify the exported state can be loaded, then generate unseen dialogue.
        restored = model.get_state_for_audio_prompt(temporary)
        generated = preview(restored, config["profiles"][name], output / f"{name}.preview.wav")
        # Audition decode-step tradeoffs without rerunning the export. These
        # samples live outside the packaged voices.
        steps = []
        for count in SAMPLE_DECODE_STEPS:
            path = SAMPLES_DIR / f"{name}.preview-d{count}.wav"
            sample = preview(restored, dict(config["profiles"][name], decode_steps=count), path)
            steps.append({"decode_steps": count, "seconds": len(sample)/model.sample_rate,
                          "file": str(path)})
        temporary.replace(output / f"{name}.safetensors")
        metadata.update({"preview_text": SAMPLE_TEXT, "preview_seconds": len(generated)/model.sample_rate,
                         "decode_step_samples": steps,
                         "synthesis_options": synthesis_options(config["profiles"][name])})
        (output / f"{name}.voice.json").write_text(json.dumps(metadata, indent=2) + "\n")
        print(f"Exported and verified {name} in {time.monotonic()-started:.1f}s", flush=True)
        print(f"{name}: decode-step samples {SAMPLE_DECODE_STEPS} in {SAMPLES_DIR}", flush=True)
    if args.activate:
        if args.config.read_bytes() != original:
            raise RuntimeError("Voice config changed during export; exported voices are ready but were not assigned")
        backup = output / f"voices.before-cloning-{time.time_ns()}.json"
        backup.write_bytes(original)
        for name in names:
            config["profiles"][name]["voice"] = f"custom/{name}.safetensors"
            config["profiles"][name].pop("cpu_threads", None)
        temporary = args.config.with_suffix(".pending.json")
        temporary.write_text(json.dumps(config, indent=2) + "\n", encoding="utf-8")
        temporary.replace(args.config)
        print(f"Assigned profiles. Previous config: {backup}", flush=True)
        print("Close the existing ForeverDubbed launcher and reopen it to load the new voices.", flush=True)


if __name__ == "__main__":
    main()
