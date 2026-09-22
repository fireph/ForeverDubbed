# ForeverDubbed

A WoW Forever addon and Go companion that read NPC and quest dialogue aloud. The addon draws a small RGB data square; the Windows app finds it anywhere on the desktop, decodes the text, and speaks using **Pocket TTS on your CPU**. The Go companion captures, decodes, selects voices, and streams audio to the Windows sound device; a small local Python helper runs the official Pocket TTS model. Playback begins with the first decoded audio instead of waiting for a complete text chunk. Speech stays on your computer. No OCR or game-memory access is used. Windows SAPI remains available as a fallback.

The square uses **16 calibrated colors** and a **48 × 48 data grid**, with a one-cell border. At the default **2 × 2 pixel cell size**, it occupies **100 × 100 physical pixels** and carries **1,124 bytes per page**. This is the only encoding format. You can enlarge cells with `/fdb cell 3` (150 × 150 pixels) if your display needs more sampling margin; the capacity stays the same. This is a custom optical format, not a standard QR code.

**Upgrading to 0.3.2:** replace the addon and `/reload`. This release adds 15,444 display-ID race/gender mappings from VoiceOver, improving detection when NPC `UnitRace` is unavailable. The 0.3.1 companion and existing Pocket TTS setup remain compatible; no model downloads or voice configuration changes are needed. If upgrading from 0.2.1, replace both addon and companion, run Pocket TTS setup once, and use `Start-ForeverDubbed.cmd` for race-based voices.

See [CHANGELOG.md](CHANGELOG.md) for earlier releases and [PROTOCOL.md](PROTOCOL.md) for the optical format.

## Quick start on Windows

**From a Git clone:** install Go 1.22 or newer, then build from the repository root before following the steps below:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build.ps1
```

This creates `dist/foreverdubbed.exe` and the addon/Windows ZIP packages. Generated files and downloaded runtimes are not committed to Git. If you already have an extracted Windows bundle with `foreverdubbed.exe`, skip the build step.

1. Copy `addon/ForeverDubbed` from the source tree or Windows bundle into the Forever client's `Interface\AddOns` directory. Alternatively extract the addon-only ZIP there. The result must be `Interface\AddOns\ForeverDubbed\ForeverDubbed.toc` (one folder level, not two).
2. Enable **ForeverDubbed** in WoW's AddOns list. Restart the client if it was running when the folder was first installed.
3. Install [uv](https://docs.astral.sh/uv/getting-started/installation/) if it is not on your PATH, then run **`Setup-PocketTTS.cmd` once**. It installs isolated Python 3.12, CPU PyTorch, Pocket TTS 3.1.0, the pinned English model, and 18 preset voices inside `.runtime`. Setup requires internet; normal use is offline. No global Python packages or CUDA installation are needed.
4. Run **`Start-ForeverDubbed.cmd`** and leave its console open. It starts the helper, waits for it to load, and runs the Go reader. Close it or press Ctrl+C to stop. Prefer WoW's windowed or borderless mode for capture.
5. Enter `/fdb test`. You should see the square, a “Found tile” message in the companion, and hear the connection test.
6. Enter `/fdb unlock`, drag the square anywhere within the game window, then `/fdb lock`. The reader finds its new position automatically. No coordinate configuration is needed.

The build produces `dist/ForeverDubbed-windows-amd64.zip`, which bundles the executable, addon, Pocket TTS scripts/config, launchers, and these instructions. Models and Python are downloaded by setup instead of being included in the ZIP. This is a console application; close it or press Ctrl+C to stop.

**Compatibility status:** the manifest targets Forever interface **16001**. The 16-color, 2-pixel transport has been confirmed working in-game. The Pocket TTS integration has been tested on Windows with real CPU synthesis and Go audio playback. Race extraction, including asynchronous display/model lookup, is covered by Lua API doubles; display/model identification should still be checked in-game with `/fdb npc`; NPCs whose race is unavailable use the configured fallback. If a later beta marks the addon out of date, inspect `/fdb status` before updating the TOC.

### Streaming speech

Rebuild the Go companion and restart `Start-ForeverDubbed.cmd` after updating both the companion and `tts/server.py`. No new models or Python dependencies are required. An older running helper must be stopped before restarting.

The helper sends mono 16-bit PCM as Pocket TTS decodes it. The companion queues a bounded number of audio buffers on one Windows playback device, including across text chunks. Changing dialogue stops and clears queued playback immediately. Pocket TTS 3.1.0 still finishes the active short generation before starting another request, discarding cancelled audio to keep model state safe.

The TTS log records `first audio in ...s` for each text chunk. This measures the helper's time to its first emitted audio, not capture/transport delay or speaker output latency.

## What it reads

- NPC gossip and quest greetings.
- Quest offers, including objectives; progress text; completion/reward text.
- NPC say, yell, whisper, and emote events, including boss dialogue.
- Item/book text when its current page becomes available.

It captures text exposed through the game API, not arbitrary text drawn by other addons. Merely selecting a quest in the quest log, cinematic subtitles, gossip option buttons, and UI text that the beta restricts are outside this version's coverage. Secret API values are skipped.

Each new message replaces the previous transmission. Rapidly clicking through dialogue or overlapping NPC chat can therefore skip earlier passages; this version follows the latest message rather than maintaining a narration backlog. A fully decoded new message interrupts the current speech. `/fdb chat` disables ambient NPC chat if you only want interaction windows.

The square remains visible for at least 15 seconds after text arrives, or three complete page cycles for very long text, then hides. Closing a quest window does not immediately hide it, allowing the reader to finish. While unlocked, it remains visible for positioning. Settings and position are stored through WoW SavedVariables when the client saves them.

## Addon controls

| Command | Behavior |
| --- | --- |
| `/fdb test` | Display and speak a connection test |
| `/fdb unlock` | Show a test square and allow dragging |
| `/fdb lock` | Save/use the position with mouse input disabled |
| `/fdb cell 2` | Set cell size to 2–8 physical pixels; default 2 |
| `/fdb reset` | Move back near the top-left corner |
| `/fdb chat` | Toggle ambient NPC/boss dialogue |
| `/fdb off` / `/fdb on` | Disable/enable transmission |
| `/fdb status` | Print version, settings, and last transmitted NPC identity/source |
| `/fdb npc` | Inspect the dialog/target NPC: race, source, gender, NPC ID, display ID, and model file ID |
| `/fdb race Skyborne` | Save a race override for the dialog/target NPC ID; reopen dialog to resend |
| `/fdb race clear` | Remove that NPC’s saved race override |

## Companion controls

Run these in PowerShell from the directory containing the executable:

```powershell
.\foreverdubbed.exe -voices
.\foreverdubbed.exe -tts sapi -voice "Microsoft Zira Desktop" -rate 1
.\foreverdubbed.exe -mute
.\foreverdubbed.exe -poll 75ms -scan 1s
.\foreverdubbed.exe -version
```

Pocket TTS is the default (`-tts pocket`; `-tts local` is an alias). Use the launcher for normal operation; invoking the EXE directly requires the helper to already be running. `-voices` lists profile IDs and the associated Pocket preset without starting the model. `-voice orc_male` forces one profile for every speaker. `-tts sapi -voices` lists installed Windows SAPI voices; `-rate -10..10` applies only to SAPI.

To hear a voice without opening WoW, run from the project/bundle root:

```powershell
.\Start-ForeverDubbed.cmd -speak-test "Welcome, traveler. Your adventure begins here." -race Orc -gender male
.\Start-ForeverDubbed.cmd -speak-test "The forest needs your help." -voice nightelf_female
```

### Race and gender voices

Edit `tts/voices.json` and restart the launcher. The table below lists the built-in preset assignments; locally cloned voices can replace individual profiles. These presets are ordinary speech voices, not custom fantasy performances. Skyborne and Blood Elves share the Night Elf presets; Draenei share the Tauren presets. There are 24 race/gender profiles plus a narrator fallback, originally using 18 distinct presets:

| Race | Male preset | Female preset |
| --- | --- | --- |
| Human | marius | alba |
| Orc | javert | cosette |
| Dwarf | george | anna |
| Night Elf | charles | fantine |
| Undead | paul | eponine |
| Tauren | jean | mary |
| Gnome | michael | azelma |
| Troll | peter_yearsley | eve |
| Skyborne | charles | fantine |
| Goblin | stuart_bell | jane |
| Blood Elf | charles | fantine |
| Draenei | jean | mary |

Selection order is `-voice`, `npc_overrides`, race/gender, then `default`. Each race has an explicit `unknown` gender fallback. Missing, empty, or unmapped races use the custom narrator for every gender, through the existing `narrator_male` profile ID. This profile uses `custom/narrator_male.safetensors` with two decoding steps. Recognized races keep their configured race/gender voices; explicit voice and NPC overrides still take precedence. The narrator fallback is separate from the Human profiles, so you can change it independently. For example, `"npc_overrides": {"4949": "orc_male"}` assigns that NPC the Orc male profile. The JSON output includes `race`, `gender`, and `npc_id` when available.

`UnitRace` frequently returns nil for NPCs even when their gender and GUID are available. The addon first uses saved NPC race assignments, public API race information, and confirmed NPC-ID mappings. Zephras Citizen (254100) is mapped to Skyborne based on in-game confirmation. Unknown NPCs are probed with an invisible `PlayerModel`. Its display ID is matched against 15,444 race/gender records from VoiceOver, then known character FileDataIDs from the [WoW file list](https://github.com/wowdev/wow-listfile) provide a model fallback. The legacy display snapshot covers 21 races including Goblin; new or reassigned Forever displays may differ, and Skyborne still need confirmed NPC assignments. See [race data and attribution](docs/RACE_DATA.md) for coverage and reproducible generation. This uses documented [SetUnit](https://github.com/Gethe/wow-ui-source/blob/forever/Interface/AddOns/Blizzard_APIDocumentationGenerated/FrameAPICharacterModelBaseDocumentation.lua) and [GetModelFileID](https://github.com/Gethe/wow-ui-source/blob/forever/Interface/AddOns/Blizzard_APIDocumentationGenerated/SimpleModelAPIDocumentation.lua) APIs. Model loads get up to 600 ms before the text is sent; a newer message cancels publication of an older pending message.

A model identifies appearance, which may differ from lore race: an elf model alone cannot prove an NPC is Skyborne. Unknown models remain unassigned. Use `/fdb npc` to inspect the evidence, or `/fdb race Skyborne` (also `Goblin`, etc.) to save the correct identity for that NPC ID. This does not reclassify every NPC that shares its model. Reopen dialogue after assigning a race.

Ambient chat uses public sender GUIDs and confirmed NPC-ID mappings, exact matches to accessible units, or the recent identity cache; it does not guess from NPC names. Secret API values are skipped. `/fdb status` shows the last transmitted NPC's race and whether it came from the API, a lookup, an override, or model appearance.

Pocket speech reads the quest title and text without announcing the speaker's name. Long passages are split into short chunks, with synthesis ahead of playback. New dialogue immediately cancels playback and discards stale audio. Pocket TTS 3.1.0 has no supported generation cancellation API, so its current short chunk finishes before the next synthesis can run.

### Local runtime and custom voices

The service binds only to `127.0.0.1:8765`; game text is never sent to an online TTS service. Startup and synthesis run with Hugging Face offline mode. Logs and the setup sample WAV are in `.runtime/pocket/`. The launcher owns and stops the helper process tree it creates; it leaves an already-running compatible helper alone. If you change the voice config, close the existing launcher and its helper before starting a new instance. The configuration is loaded at startup; opening a second launcher does not refresh the first helper.

The default setup uses the public preset model, so no Hugging Face login is required. For custom voices, `tts/clone_voice.py` accepts WAV or MP3 recordings from `audio_clips/`, prepares reference excerpts, exports reusable `.safetensors` voice states, and generates preview dialogue before optionally assigning profiles. Export requires access to Kyutai's cloning-enabled model; audio processing and synthesis remain local. See [the voice cloning guide](docs/VOICE_CLONING.md) for the Human and Night Elf commands, mixed audio formats, and manual excerpt selection. Profiles also accept local `.wav` or `.safetensors` paths relative to `voices.json`. Voice states must match the selected model (`english_2026-04`). After adding a preset, rerun setup to cache it before offline use.

Upstream documentation: [Pocket TTS](https://github.com/kyutai-labs/pocket-tts), [Python API](https://github.com/kyutai-labs/pocket-tts/blob/main/docs/API%20Reference/python-api.md), and [preset voice sources/licenses](https://huggingface.co/kyutai/tts-voices).

Finished `tts/custom/*.safetensors` voice states can be committed alongside `tts/voices.json` and are included in Windows bundles when referenced by that configuration. Source recordings, previews, local backups, credentials, and base-model downloads remain Git-ignored. A fresh checkout still needs the normal Pocket TTS setup; reference recordings and export metadata are not required to play the saved voices.

Decoded messages are printed as one JSON object per line on stdout. Discovery and speech diagnostics go to stderr. To save text without speech:

```powershell
.\foreverdubbed.exe -mute > dialogue.jsonl
```

The reader scans all monitors about once per second until it finds the square. It then samples only that region every 75 ms. Four failed reads trigger rediscovery, including after dragging, resizing, or moving the game window. Only one visible addon instance is supported at a time. Full desktop scans stay in memory; screenshots are neither saved nor uploaded.

For a diagnostic screenshot, explicitly run `.\foreverdubbed.exe -snapshot capture.png`. It waits three seconds, saves one PNG of the entire desktop, reports whether it found a valid tile, and exits. Keep WoW visible, use `/fdb unlock` so the square stays displayed, and move the pointer and other windows away from it. The PNG can be inspected locally or passed to `-image capture.png`. The capture includes other visible windows. The normal reader never saves images.

## Troubleshooting

- **No square:** `/fdb on`, `/fdb reset`, then `/fdb test`. Check `/fdb status` and that the addon appears in WoW's AddOns list. `/console scriptErrors 1` enables Lua error reporting.
- **Square visible but not found:** check that both the addon and companion were updated, and `/fdb status` reports addon 0.3.2. Try `/fdb unlock` and `/fdb cell 2`, move the mouse away, and ensure the entire border is visible. Test in borderless/windowed mode. The reader reports whether there was no finder pattern or whether a candidate failed its palette, border, or checksum checks. If needed, try `/fdb cell 3` or save a diagnostic PNG. The palette is calibrated from the screen, but severe HDR/color filters, blur, or clipped colors can still prevent reading. The decoder supports integer cell sizes of 2–8 desktop pixels; externally scaling or compressing the game image can prevent decoding.
- **Found but no speech:** launch using `Start-ForeverDubbed.cmd`, check `.runtime/pocket/server-errors.log`, and test with `-speak-test`. Ensure `-mute` is absent and check Windows' default audio output. `-tts sapi` bypasses Pocket TTS for comparison.
- **“An existing Pocket TTS helper uses different voices”:** the running helper has an older copy of `tts/voices.json`. Close the original launcher with Ctrl+C, then reopen it. If you started `tts/server.py` manually, stop that server too. The launcher checks the full configuration and does not reload an existing helper automatically.
- **Missing local model/voice:** rerun `Setup-PocketTTS.cmd` while online, then restart the launcher. The helper intentionally cannot download missing files during play.
- **Square disappears:** normal after the transmission timeout; `/fdb test` sends again. `/fdb unlock` keeps it displayed during setup.
- **Long text takes time:** each page carries 1,124 bytes and lasts 250 ms. A 2 KB message needs two pages, or 0.5 seconds per cycle, plus discovery and TTS startup. Missed pages are recovered on later cycles. Text is spoken only after every page passes validation.
- **Changing quest screens interrupts narration:** intentional latest-message behavior. Ambient NPC speech can also interrupt; toggle it with `/fdb chat`.

The border includes a reference swatch for every palette color. The decoder measures these on every captured page and compares data cells to the observed colors, adapting to uniform gamma and color changes without assuming exact screen RGB values. Ambiguous or insufficiently separated colors are rejected. This improves tolerance; it does not make the transport immune to all display processing.

There is no forward error correction. Per-page and whole-message Adler-32 checksums reject most accidental damage, and repeated pages provide retries. Checksums are for accidental corruption, not authentication. The companion accepts a valid tile anywhere visible on the desktop.

## Build and test

The Go executable requires Go 1.22 or newer to build and has no third-party Go dependencies. Lua 5.1 or newer is needed for the optional cross-language/addon smoke tests, not for building the companion or using the addon.

```powershell
go test ./...
go vet ./...
$env:LUA = "C:\path\to\lua.exe"
go test ./... -count=1
go build -buildvcs=false -trimpath -o dist/foreverdubbed.exe ./cmd/foreverdubbed
```

After installing Pocket TTS, run the helper tests separately. They use a mocked model and do not download weights or play audio:

```powershell
.\.runtime\pocket-env\Scripts\python.exe -m unittest discover -s tts -p "test_*.py"
```

On Windows, `powershell -NoProfile -File scripts/build.ps1` builds the executable and both ZIP packages. From Linux, cross-compile with:

```sh
GOOS=windows GOARCH=amd64 go build -buildvcs=false -trimpath -o dist/foreverdubbed.exe ./cmd/foreverdubbed
LUA=/path/to/lua go test -race ./...
python -m unittest discover -s tests -p 'test_*.py'
```

The portable decoder can read PNGs on any OS. For a paged message, supply one unmodified screenshot of each distinct page, in any order:

```sh
go run ./cmd/foreverdubbed -image page1.png,page2.png,page3.png
```

Tests cover voice routing and overrides, cancellation, HTTP service identity/config checks, PCM WAV validation, speaker metadata, the Lua/Go byte and palette contract, Unicode spanning pages, out-of-order/duplicate pages, session changes, sequence wraparound, invalid dimensions, corruption, gamma/tint/noise transforms, damaged reference swatches, moved tiles, negative monitor coordinates, missing NPC races, display lookup precedence, model-load timing and stale identities, saved race assignments, settings migration, physical pixel sizing at multiple resolutions/UI scales, and all supported cell sizes. Lua tests use mocked game APIs; they do not substitute for testing the real client. Tests explicitly skip the Lua checks if an interpreter is unavailable.

See [PROTOCOL.md](PROTOCOL.md) for the wire format. API references used: [Forever gossip API source](https://github.com/Gethe/wow-ui-source/blob/forever/Interface/AddOns/Blizzard_APIDocumentationGenerated/GossipInfoDocumentation.lua), [Forever quest UI source](https://github.com/Gethe/wow-ui-source/blob/forever/Interface/AddOns/Blizzard_UIPanels_Game/Mainline/QuestFrame.lua), [Windows BitBlt](https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-bitblt), and [SpeechSynthesizer.Speak](https://learn.microsoft.com/en-us/dotnet/api/system.speech.synthesis.speechsynthesizer.speak?view=netframework-4.8.1).
