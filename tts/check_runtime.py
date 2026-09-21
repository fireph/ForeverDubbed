"""Verify the isolated Windows CPU runtime."""
from importlib.metadata import version
import torch

print(f"Pocket TTS {version('pocket-tts')}, PyTorch {torch.__version__}, CPU runtime")
