"""Verify the isolated Windows CPU runtime."""
from importlib.metadata import version
import torch
import soundfile as sf

print(f"Pocket TTS {version('pocket-tts')}, PyTorch {torch.__version__}, CPU runtime")
if "MP3" not in sf.available_formats():
    raise RuntimeError("The installed SoundFile decoder lacks MP3 support")
print(f"SoundFile {sf.__version__}, libsndfile {sf.__libsndfile_version__}: WAV/MP3 decoding ready")
