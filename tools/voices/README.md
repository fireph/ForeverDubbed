# Voice export tools

These optional Python tools prepare reference audio and export saved voices for the native app. They run on CPU; the app itself does not need Python.

Install [uv](https://docs.astral.sh/uv/getting-started/installation/), then run these commands **from the repository root** (macOS/Linux or Windows PowerShell):

```sh
uv sync --project tools/voices --locked
uv run --project tools/voices --locked tools/voices/clone_voice.py --help
```

uv selects Python 3.12, creates `tools/voices/.venv`, and installs the versions in `uv.lock`. No environment activation or separate pip/torch installation is needed. `uv run` also performs setup automatically, so the initial `uv sync` is optional. The pinned environment supports Apple Silicon macOS, Windows x64, and Linux x64/arm64. PyTorch 2.9.1 does not provide Intel macOS wheels.

## Prepare, export, and check

Prepare a 20-second reference without downloading or loading a model:

```sh
uv run --project tools/voices --locked tools/voices/clone_voice.py --voice undead_male=audio_clips/undead-male.wav --seconds 20 --prepare-only
```

Listen to `tts/custom/undead_male.reference.wav`. Add `--start 10` to select a particular excerpt. Input paths are relative to your current directory; quote the whole `PROFILE=AUDIO` argument when the path contains spaces. Reference preparation leaves existing exported voices and configuration intact.

For cloning, accept access to the [Kyutai model](https://huggingface.co/kyutai/pocket-tts) if required, then authenticate interactively:

```sh
uv run --project tools/voices --locked hf auth login
```

Download/cache the pinned English model and check a preview (optional; cloning with `--online` can download it directly):

```sh
uv run --project tools/voices --locked tools/voices/prepare.py
```

Export, preview, and assign a replacement voice:

```sh
uv run --project tools/voices --locked tools/voices/clone_voice.py --voice undead_male=audio_clips/undead-male.wav --seconds 20 --online --force --activate
```

`--force` allows replacing the existing saved voice. Omit `--activate` to export without changing the profile assignment. After the model is cached, omit `--online` to keep model loading offline. See [the export guide](../../docs/VOICE_CLONING.md) for native playback validation and output details.

Model downloads and the setup sample live in `.runtime/pocket`; reference audio, previews, and saved states live in `tts/custom` (or beside a custom `--config`). Existing Hugging Face CLI credentials are reused; login tokens are not included in releases. `uv sync` installs Python packages only and does not download model weights.

## Tests and dependencies

Run the audio preparation and configuration tests without model weights or a Hugging Face login:

```sh
uv run --project tools/voices --locked python -m unittest discover -s tools/voices -p 'test_*.py'
```

`pyproject.toml` is the dependency source of truth; commit it and `uv.lock` together. After editing dependencies, run `uv lock --project tools/voices`, then rerun the tests. Python 3.12 is selected by `.python-version`. The explicit [PyTorch CPU index](https://docs.astral.sh/uv/guides/integration/pytorch/) is used only for torch on Windows/Linux; macOS uses PyPI. No CUDA packages are needed.
