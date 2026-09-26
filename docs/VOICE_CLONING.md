# Exporting custom voices

Python is an optional **development-time export tool only**. The application plays saved `.safetensors` states with PocketTTS.cpp and never starts Python. Existing custom states remain usable with the pinned `english_2026-04` model.

## Prepare an export environment

Install [uv](https://docs.astral.sh/uv/getting-started/installation/). From the repository root:

```sh
uv sync --project tools/voices --locked
uv run --project tools/voices --locked tools/voices/clone_voice.py --help
```

The project pins Python 3.12 and locks its Python dependencies. uv creates `tools/voices/.venv` automatically; no activation or separate PyTorch installation is needed. Windows/Linux use CPU-only PyTorch wheels; Apple Silicon macOS uses the PyPI wheel. The pinned PyTorch release does not support Intel macOS. See [the tools README](../tools/voices/README.md) for reference-only preparation, model-cache setup, and tests.

Cloning requires access to the official [Kyutai cloning-enabled weights](https://huggingface.co/kyutai/pocket-tts). Accept access if required, then authenticate:

```sh
uv run --project tools/voices --locked hf auth login
```

The tools reuse your Hugging Face CLI login and keep downloaded models in `.runtime/pocket`. Credentials, caches, and the Python environment are not included in the application bundle.

## Export and assign a voice

Run from the repository root:

```sh
uv run --project tools/voices --locked tools/voices/clone_voice.py --voice undead_male=audio_clips/undead-male.wav --seconds 20 --online --force --activate
```

The example replaces the existing bundled voice with `--force`. Omit that flag when creating a new voice. Use `--start` to choose an excerpt, `--prepare-only` to inspect the reference first, and `--force` to intentionally replace an existing state. WAV and MP3 sources are supported. `--decode-steps 4` is retained in the profile and honored by native synthesis. `--cpu-threads` controls the Python preview only; the application's thread budget is set with its `-cpu-threads` flag.

Exports go to `tts/custom/<profile>.safetensors`. The tool checks a Python preview before activating a profile, but also validate the final native result:

```sh
go run -tags pocket_native ./tools/voicecheck -voice undead_male
```

The native test creates `.runtime/voice-samples/undead_male.wav`. Listen to it before distributing the voice. `tts/voices.json` stores custom paths relative to itself and requires forward slashes on every platform, for example `custom/undead_male.safetensors`. Backslashes are rejected by the app, release builder, and Python tools. Runtime profiles must reference preset names or exported `.safetensors` files. The release builder rejects profiles still pointing to WAV/MP3 references. Source recordings, reference WAVs, previews, and backups are excluded from release packages.

Do not load states from another checkpoint. April-model exported states include BOS-before-voice conditioning; the native adapter restores them directly. See [native runtime/model details](../native/README.md) for ONNX exports and upstream issue #12.
