# Exporting custom voices

Python is an optional **development-time export tool only**. The application plays saved `.safetensors` states with PocketTTS.cpp and never starts Python. Existing custom states remain usable with the pinned `english_2026-04` model.

## Prepare an export environment

Install uv and create a separate environment:

```sh
uv venv --python 3.12 .runtime/export-env
```

On Windows, the interpreter is `.runtime/export-env/Scripts/python.exe`; on Linux/macOS it is `.runtime/export-env/bin/python`. Install `tools/voices/requirements.txt` using `uv pip install --python <interpreter> -r tools/voices/requirements.txt`. For CPU-only Windows/Linux builds, install `torch==2.9.1` first from `https://download.pytorch.org/whl/cpu` using uv's `--index-url` option. macOS uses the normal PyPI wheel.

Cloning requires access to the official Kyutai cloning-enabled weights. Authenticate using Hugging Face's CLI if necessary. Credentials and downloads stay in the export environment/cache; nothing is included in the application bundle.

## Export and assign a voice

Run with the export environment's interpreter:

```sh
python tools/voices/clone_voice.py --voice undead_male=audio_clips/undead-male.wav --seconds 20 --online --activate
```

Use `--start` to choose an excerpt, `--prepare-only` to inspect the reference first, and `--force` to intentionally replace an existing state. WAV and MP3 sources are supported. `--decode-steps 4` is retained in the profile and honored by native synthesis. `--cpu-threads` controls the Python preview only; the application's thread budget is set with its `-cpu-threads` flag.

Exports go to `tts/custom/<profile>.safetensors`. The tool checks a Python preview before activating a profile, but also validate the final native result:

```sh
go run -tags pocket_native ./tools/voicecheck -voice undead_male
```

The native test creates `.runtime/voice-samples/undead_male.wav`. Listen to it before distributing the voice. `tts/voices.json` stores custom paths relative to itself; source recordings, reference WAVs, previews, and backups are not needed for runtime playback and are excluded from release packages.

Do not load states from another checkpoint. April-model exported states include BOS-before-voice conditioning; the native adapter restores them directly. See [native runtime/model details](../native/README.md) for ONNX exports and upstream issue #12.
