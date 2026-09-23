"""Download the pinned English model and the configured preset voice states."""
import argparse
import json
import time

from runtime import MODEL, ROOT, RUNTIME, load_model, load_profiles, voice_source


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", type=str, default=str(ROOT / "tts" / "voices.json"))
    args = parser.parse_args()
    print(f"Downloading {MODEL} and preset voices...", flush=True)
    model = load_model(offline=False)
    sources = {voice_source(args.config, p) for p in load_profiles(args.config).values()}
    for source in sorted(sources, key=str):
        model.get_state_for_audio_prompt(source)
        print(f"Cached voice: {source}", flush=True)
    print("Checking synthesis...", flush=True)
    started = time.monotonic()
    state = model.get_state_for_audio_prompt("alba")
    audio = model.generate_audio(state, "Welcome, traveler. Your adventure begins here.")
    import numpy as np
    import scipy.io.wavfile
    RUNTIME.mkdir(parents=True, exist_ok=True)
    scipy.io.wavfile.write(RUNTIME / "sample.wav", model.sample_rate,
                          (audio.numpy().clip(-1, 1) * 32767).astype(np.int16))
    print(f"Generated {len(audio)/model.sample_rate:.2f}s audio in {time.monotonic()-started:.2f}s on CPU", flush=True)
    (RUNTIME / "ready.json").write_text(json.dumps({"model": MODEL,
        "voice_cloning": model.has_voice_cloning}), encoding="utf-8")
    print("Python export environment ready. The application uses the native ONNX runtime.", flush=True)


if __name__ == "__main__":
    main()
