# Custom voices with Pocket TTS

ForeverDubbed uses Pocket TTS 3.1.0 and the `english_2026-04` model. The default setup downloads public preset-only weights. Existing voice profiles accept local WAV or `.safetensors` paths, but the project does not yet provide an automated cloning wizard.

## Prepare a reference

Choose a clean excerpt containing one speaker and complete sentences. Start with 10–20 seconds, and compare different excerpts if you have a longer recording. Avoid music, combat sounds, overlapping speakers, strong reverb, and long silence where possible; reference audio quality and delivery affect the result.

Pocket's `export-voice` command uses only the first 30 seconds. A minute-long recording is useful for selecting excerpts, but passing the whole recording will not use all of it. No transcript or model training is required for this workflow.

Export the selected excerpt as a mono, 16-bit PCM WAV, optionally at 24 kHz, such as `tts/custom/skyborne_male.wav`. The installed runtime reads that format without extra audio dependencies. Direct MP3 loading requires the optional `soundfile` dependency; converting to WAV avoids that extra step.

`tts/custom/`, audio recordings, and model weights are ignored by Git. Keep your own backup of custom references and exported voices; they are not part of the source repository or release bundle.

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
