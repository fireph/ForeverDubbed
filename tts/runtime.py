"""Project-local Pocket TTS cache and CPU model loading."""
import json
import os
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
RUNTIME = ROOT / ".runtime" / "pocket"
LANGUAGE = "english_2026-04"
MODEL = "Pocket TTS 3.1.0 / " + LANGUAGE
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
    return profiles


def voice_source(config_path, profile):
    source = profile["voice"]
    # Future custom WAV/safetensors references are relative to the config file.
    if Path(source).suffix.lower() in (".wav", ".safetensors"):
        return Path(config_path).resolve().parent / source
    if not source.replace("_", "").isalnum():
        raise ValueError("Use a preset name or a local WAV/safetensors file")
    return source
