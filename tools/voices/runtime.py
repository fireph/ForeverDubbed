"""Project-local Pocket TTS cache and CPU model loading."""
import json
import os
from contextlib import contextmanager
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
RUNTIME = ROOT / ".runtime" / "pocket"
LANGUAGE = "english_2026-04"
MODEL = "Pocket TTS 3.1.0 / " + LANGUAGE
# Reuse an existing CLI login while keeping model downloads project-local.
# Explicit token settings and a project-local login take precedence.
_login_home = Path(os.environ.get("HF_HOME", str(
    Path(os.environ.get("XDG_CACHE_HOME", str(Path.home() / ".cache"))) / "huggingface")))
if not (RUNTIME / "huggingface" / "token").is_file() and (_login_home / "token").is_file():
    os.environ.setdefault("HF_TOKEN_PATH", str(_login_home / "token"))
os.environ["HF_HOME"] = str(RUNTIME / "huggingface")
os.environ["HF_HUB_DISABLE_SYMLINKS_WARNING"] = "1"
os.environ["HF_HUB_DISABLE_TELEMETRY"] = "1"


def load_model(offline=True):
    # Set before importing huggingface_hub: it reads the setting at import time.
    os.environ["HF_HUB_OFFLINE"] = "1" if offline else "0"
    from pocket_tts import TTSModel
    import torch

    torch.set_num_threads(1)
    return TTSModel.load_model(language=LANGUAGE)


def load_profiles(path):
    config = json.loads(Path(path).read_text(encoding="utf-8"))
    profiles = config["profiles"]
    for name, profile in profiles.items():
        if not isinstance(profile.get("voice"), str) or not profile["voice"]:
            raise ValueError(f"Profile {name} needs a Pocket TTS voice name")
        synthesis_options(profile)
    return profiles


def synthesis_options(profile):
    """Validate optional per-voice compute settings before loading a model."""
    options = {}
    for key, maximum in (("decode_steps", 64), ("cpu_threads", os.cpu_count() or 1)):
        if key not in profile:
            continue
        value = profile[key]
        if type(value) is not int or not 1 <= value <= maximum:
            raise ValueError(f"{key} must be an integer between 1 and {maximum}")
        options[key] = value
    return options


@contextmanager
def synthesis_settings(model, profile):
    # The service holds its synthesis lock throughout this context. Restore all
    # changes even after errors so one voice never affects another's settings.
    options = synthesis_options(profile)
    if not options:
        yield
        return
    import torch
    old_steps, old_threads = model.sampler_decode_steps, torch.get_num_threads()
    try:
        model.sampler_decode_steps = options.get("decode_steps", old_steps)
        if "cpu_threads" in options and options["cpu_threads"] != old_threads:
            torch.set_num_threads(options["cpu_threads"])
        yield
    finally:
        model.sampler_decode_steps = old_steps
        if torch.get_num_threads() != old_threads:
            torch.set_num_threads(old_threads)


def voice_source(config_path, profile):
    source = profile["voice"]
    # Custom WAV/safetensors references are relative to the config file.
    if Path(source).suffix.lower() in (".wav", ".safetensors"):
        return Path(config_path).resolve().parent / source
    if not source.replace("_", "").isalnum():
        raise ValueError("Use a preset name or a local WAV/safetensors file")
    return source
