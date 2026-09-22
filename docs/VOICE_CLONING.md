# Custom voices with Pocket TTS

ForeverDubbed uses Pocket TTS 3.1.0 and the `english_2026-04` model. The default setup downloads public preset-only weights. `tts/clone_voice.py` accepts local WAV and MP3 recordings, prepares references, exports reusable voice states, and generates preview dialogue. No transcript or training run is required. Audio stays on your machine.

## Quick start with the human recordings

Accept access on the [Kyutai model page](https://huggingface.co/kyutai/pocket-tts), then log in from PowerShell in the project root (enter your token only in the local prompt):

```powershell
$env:HF_HOME = Join-Path (Get-Location).Path '.runtime/pocket/huggingface'
.\.runtime\pocket-env\Scripts\hf.exe auth login
.\.runtime\pocket-env\Scripts\python.exe tts/clone_voice.py --voice human_male=audio_clips/human_male.wav --voice human_female=audio_clips/human_female.wav --online --activate
```

If you already logged in using the standard Hugging Face CLI cache, the runtime can reuse that login while still storing downloaded models under `.runtime/`. Explicit token settings and a project-local login take precedence.

The tool selects roughly 20 seconds with active audio, converts it to mono 24 kHz PCM, and writes these files for each profile under `tts/custom/`:

- `human_male.reference.wav`: the prepared reference (and similarly for female).
- `human_male.safetensors`: the reusable voice state.
- `human_male.preview.wav`: newly synthesized dialogue for auditioning.
- Reference and voice JSON records: source hash, selected times, model version, and preview text.

Automatic selection uses audio energy; it cannot distinguish good acting from music, combat sounds, or another speaker. Listen to the reference and preview. To select your own excerpt, run one `--voice` with `--start 5 --seconds 15`. Add `--force` to replace an existing exported voice. `--prepare-only` prepares references without model access; omit `--activate` to export and audition without assigning profiles. Omit `--online` once the cloning weights are cached.

Activation backs up the configuration under `tts/custom/` and changes only the requested profiles after all exports and previews succeed. It fails if the configuration changed during generation. Close the existing ForeverDubbed launcher and restart it after activation; the helper checks the configuration digest. The existing female fallback also uses `human_female`, so assigning it changes that fallback voice too. Finished `.safetensors` files directly under `tts/custom/` can be committed with `tts/voices.json`; release builds include the local voice files referenced by that configuration. Recordings, previews, metadata, experiments, backups, and unfinished exports remain ignored.

## WAV and MP3 inputs

You can mix `.wav` and `.mp3` files in `audio_clips/` and in the same export command. File extensions are case-insensitive. Use the configured profile name before `=` and the actual recording path after it; filenames can contain hyphens or underscores. Quote the entire `PROFILE=PATH` argument if the path contains spaces.

For the Night Elf recordings, with cloning weights already cached:

```powershell
.\.runtime\pocket-env\Scripts\python.exe tts/clone_voice.py --voice nightelf_male=audio_clips/nightelf-male.mp3 --voice nightelf_female=audio_clips/nightelf-female.mp3 --activate
```

The decoder converts either format to the same mono 24 kHz WAV reference internally. Your original files are preserved; previews remain WAV and exported voices remain `.safetensors`. MP3 decoding uses the pinned `soundfile` dependency from `tts/requirements.txt`, including its bundled Windows decoder; no separate FFmpeg installation is required. Existing installations can add it with:

```powershell
uv pip install --python .runtime/pocket-env/Scripts/python.exe soundfile==0.13.1
```

## Spending more compute on a voice

`--decode-steps` sets the decoding work used for both the preview and, with
`--activate`, subsequent in-game synthesis. Pocket's default is 1 step. For
example, to rebuild only Orc male from a short WAV using 32 steps:

```powershell
.\.runtime\pocket-env\Scripts\python.exe tts/clone_voice.py --voice orc_male=audio_clips/orc-male.wav --start 0 --seconds 30 --decode-steps 32 --cpu-threads 1 --force --activate
```

This uses the complete recording if it is shorter than 30 seconds. The exported
voice state contains the reference conditioning; decoding work is performed each
time speech is generated. The activated `orc_male` profile stores `decode_steps`
and `cpu_threads`, so restarting the helper applies the same settings in-game.
Other profiles retain their own settings. Accepted values are 1–64 steps and
1–the machine's logical CPU count for threads. More steps cost time and may help,
but do not guarantee better speech or fix a poorly reproduced voice. More CPU
threads can actually be slower; a local benchmark should guide this setting.

The default model already uses full precision without int8 quantization. These
options increase synthesis work, not model training. To return to the standard
speed, set `decode_steps` to 1 or remove it from the profile, then restart the
launcher. The generation settings are restored between requests so an Orc
profile's settings do not affect other voices.

The sections below cover manual preparation and the upstream export command.

## Prepare a reference

Choose a clean excerpt containing one speaker and complete sentences. Start with 10–20 seconds, and compare different excerpts if you have a longer recording. Avoid music, combat sounds, overlapping speakers, strong reverb, and long silence where possible; reference audio quality and delivery affect the result.

Pocket's `export-voice` command uses only the first 30 seconds. A minute-long recording is useful for selecting excerpts, but passing the whole recording will not use all of it. No transcript or model training is required for this workflow.

For manual preparation, export the selected excerpt as a mono, 16-bit PCM WAV, optionally at 24 kHz, such as `tts/custom/skyborne_male.wav`. Alternatively, pass your WAV or MP3 directly to `tts/clone_voice.py` and let it prepare the reference.

For playback on another machine, commit the finished `tts/custom/*.safetensors`
files together with `tts/voices.json` and run the normal setup there. The original
recordings and `.reference.json`/`.voice.json` metadata are not needed for playback.
The matching Pocket TTS base model and dependencies are still downloaded by setup;
they are not included in Git. Keep a separate backup of your source recordings.

Git ignores `audio_clips/`, custom WAV/MP3 files, metadata, backup and comparison
subdirectories, `*.pending.safetensors`, and all of `.runtime/` (including cached
models and credentials). Only finished voice states directly under `tts/custom/`
are exempted from the general model-weight ignore rule. `scripts/build.ps1`
copies the configured local voice files into the Windows bundle, without copying
the rest of the custom directory.

## Enable cloning and export the voice

First run `Setup-PocketTTS.cmd` to install the normal runtime. Accept the access conditions on the [Kyutai Pocket TTS model page](https://huggingface.co/kyutai/pocket-tts) using your Hugging Face account. Then run these commands in PowerShell from the ForeverDubbed project or bundle root:

```powershell
$env:HF_HOME = Join-Path (Get-Location).Path '.runtime/pocket/huggingface'
$env:HF_HUB_OFFLINE = '0'
.\.runtime\pocket-env\Scripts\hf.exe auth login
New-Item -ItemType Directory -Force tts/custom | Out-Null
```

Authenticate through the local CLI prompt. The project-local Hugging Face cache, including its credentials, is under the ignored `.runtime/` directory. Place the prepared WAV in `tts/custom/`, then export it:

```powershell
.\.runtime\pocket-env\Scripts\python.exe -m pocket_tts export-voice --language english_2026-04 tts/custom/skyborne_male.wav tts/custom/skyborne_male.safetensors
```

This downloads the cloning-enabled weights if necessary and creates a reusable voice state. If access fails, Pocket may fall back to preset-only weights and then reject the custom audio; verify model access and authentication before retrying. The export and subsequent synthesis run locally. Use the same model version for voice export and playback.

## Assign and audition it

In the existing `profiles` object in `tts/voices.json`, replace only the desired profile:

```json
"skyborne_male": {
  "voice": "custom/skyborne_male.safetensors"
}
```

The path is relative to `tts/voices.json`. Keep the existing `races.skyborne.male` mapping to `skyborne_male`. Use a separate reference/profile for a different speaker, such as the female Skyborne voice.

Close the existing ForeverDubbed launcher and its helper, then audition the profile:

```powershell
.\Start-ForeverDubbed.cmd -speak-test "Welcome, traveler. The winds have brought you to Zephras." -voice skyborne_male
```

Test several passages that were not in the reference recording. Compare exported voices from different excerpts before choosing one. The saved `.safetensors` state avoids re-encoding the reference every time the helper starts; normal startup and gameplay remain offline.

Cloning selects how text sounds. It does not identify NPC races: use the addon's existing race lookup or `/fdb race Skyborne` when an NPC needs an explicit assignment.

References: [Pocket TTS export command](https://github.com/kyutai-labs/pocket-tts/blob/main/docs/CLI%20Commands/export_voice.md), [Python voice-state API](https://github.com/kyutai-labs/pocket-tts/blob/main/docs/API%20Reference/python-api.md), and [upstream project](https://github.com/kyutai-labs/pocket-tts).
