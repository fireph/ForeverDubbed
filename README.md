# ForeverDubbed

A WoW Forever addon and Go companion that read NPC and quest dialogue aloud. The addon draws a small RGB data square; the Go app finds it on the Windows desktop or inside the WoW window on macOS, decodes the text, and speaks using **Pocket TTS on your CPU**. The Go companion captures, decodes, selects voices, and streams audio through an in-process PocketTTS.cpp engine. No Python interpreter or local HTTP service is used at runtime. Speech stays on your computer. No OCR or game-memory access is used. Windows SAPI and macOS system voices remain available as fallbacks.

The square uses **16 subtle dark-navy calibrated colors** and a **48 × 48 data grid**, with a one-cell calibration ring and a **light-blue 2px outer outline**. At the default **2 × 2 pixel cell size**, it occupies **104 × 104 physical pixels** and carries **1,056 bytes per page**. The outline stays 2 physical pixels thick at every cell size. A crisp light-blue sine wave with a 2-cell stroke measured perpendicular to the curve drifts left through the middle half of the data area, animating at 15 fps and completing a loop every 3.2 seconds. Its hard-coded shape moves one whole cell per frame with edge wrapping, so the stroke never changes shape during motion. Both encoder and decoder skip its cells. This FDB5 format requires updating both the addon and companion to 0.5.0. You can enlarge cells with `/fdb cell 3` (154 × 154 pixels) if your display needs more sampling margin; the capacity stays the same. This is a custom optical format, not a standard QR code.

**Native runtime migration:** rebuild the companion and bundle the native runtime/model files below. Existing April-model `.safetensors` voices are preserved. The old Python service and setup scripts are no longer used.

See [CHANGELOG.md](CHANGELOG.md) for earlier releases and [PROTOCOL.md](PROTOCOL.md) for the optical format.

## macOS

The companion supports macOS 14+ on Apple Silicon and Intel. macOS captures only the World of Warcraft Beta.app game window, including for `-snapshot`; it waits when the game is unavailable and never falls back to the desktop. You can build the full macOS release **on Linux**, including in GitHub Actions, using the pinned OSXCross setup. See [macOS builds and setup](docs/macos.md) for commands, Screen Recording permission, and validation steps. Linux cross-compilation checks the native code; live capture/audio still need testing on a Mac.

## Quick start on Windows

From a release ZIP, extract the entire folder and run `foreverdubbed.exe` (or the optional `Start-ForeverDubbed.cmd`). Keep the ONNX Runtime DLLs, `native/`, and `tts/` beside the executable. Python, uv, and Go are not needed to run a prepared release.

From source, install Go 1.22+, CMake 3.28+, Git, and an x64 MinGW-w64 C/C++17 toolchain (GCC/G++ on PATH), then run:

```sh
go run ./tools/native
go run ./tools/models
go run ./tools/build
```

This prepares Windows x64 native dependencies, downloads pinned model/preset assets, compiles PocketTTS.cpp into the Go executable with cgo, and produces `dist/foreverdubbed.exe` and the addon/Windows ZIPs. See [native build details](native/README.md) for Windows toolchains and cross-host packaging.

To build the same Windows release from **Ubuntu 24.04+/WSL**, install the build prerequisites once, then run those same three Go commands from the repository root:

```sh
sudo apt-get update
sudo apt-get install golang-go cmake git build-essential g++-mingw-w64-x86-64-posix
go run ./tools/native
go run ./tools/models
go run ./tools/build
```

The tools automatically select MinGW-w64 and download Windows DLLs even when running in Ubuntu. Run the resulting app in Windows, or extract `dist/ForeverDubbed-windows-amd64.zip` there. These commands produce the Windows release; for macOS releases from Linux, follow [the macOS guide](docs/macos.md).

1. Copy `addon/ForeverDubbed` into the Forever client's `Interface\AddOns` directory and enable it in the game.
2. Run the companion. It loads the native engine directly; there is no separate service to start.
3. Enter `/fdb test`. You should see the tile detected and hear the connection test.
4. Use `/fdb unlock`, move the tile, then `/fdb lock`. Prefer windowed or borderless game mode.

The release contains one application executable plus ONNX Runtime libraries, ONNX models, and `.safetensors` voices. It is not a single statically linked binary. Desktop capture/playback support Windows and macOS 14+; the native voice-test tool also runs on Linux.

### Streaming speech

Streaming is always enabled. The decoder generates audio in batches of up to 1.2 seconds, including the first batch; shorter sentence endings are flushed immediately. Playback starts when the first batch is ready, so startup delay depends on generation speed. Audio is queued in at most 100 ms buffers for responsive cancellation. New dialogue stops playback and cancels native generation. The model is reused after its workers finish, preserving safe voice changes.

Generate clips and measure first-audio latency with `go run -tags pocket_native ./tools/voicecheck`. Samples and the timing report go to `.runtime/voice-samples/`. These times measure PCM availability after model loading, not screen capture or speaker latency.

## What it reads

- NPC gossip and quest greetings.
- Quest offers, including objectives; progress text; completion/reward text.
- NPC say, yell, whisper, and emote events, including boss dialogue.
- Item/book text when its current page becomes available.

It captures text exposed through the game API, not arbitrary text drawn by other addons. Merely selecting a quest in the quest log, cinematic subtitles, gossip option buttons, and UI text that the beta restricts are outside this version's coverage. Secret API values are skipped.

Each new message replaces the previous transmission. Rapidly clicking through dialogue or overlapping NPC chat can therefore skip earlier passages; this version follows the latest message rather than maintaining a narration backlog. A fully decoded new message interrupts the current speech. `/fdb chat` disables ambient NPC chat if you only want interaction windows.

The addon keeps the wave at 15 fps and switches text pages every 250 ms. It caches the 48 wave layouts, reuses a page decoding buffer, and updates only textures whose colors changed, reducing Lua work and texture updates. Unused data cells contain a cached noise pattern so the texture fills the square even for short messages. The receiver ignores this padding when reading the text; existing FDB5 companions remain compatible.

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

Pocket TTS is the default (`-tts pocket`; `-tts local` is an alias). The EXE loads the native engine automatically; no launcher or service is required. `-voices` lists profile IDs and the associated Pocket preset without starting the model. `-voice orc_male` forces one profile for every speaker. `-tts sapi -voices` lists installed Windows SAPI voices; `-tts system` selects OS voices on either platform; `-rate -10..10` applies to system voices.

To hear a voice without opening WoW, run from the project/bundle root:

```powershell
.\Start-ForeverDubbed.cmd -speak-test "Welcome, traveler. Your adventure begins here." -race Orc -gender male
.\Start-ForeverDubbed.cmd -speak-test "The forest needs your help." -voice nightelf_female
```

### Race and gender voices

Edit `tts/voices.json` and restart the companion. The 25 race/gender profiles currently use 13 custom states plus eight distinct presets. `-voices` lists the exact current assignments. Custom states live under `tts/custom/`; preset states live under `native/presets/` in a release. Optional `decode_steps` settings are preserved by the native engine.

Selection order is `-voice`, `npc_overrides`, race/gender, then `default`. Each race has an explicit `unknown` gender fallback. Missing, empty, or unmapped races use the custom narrator for every gender, through the existing `narrator_male` profile ID. This profile uses `custom/narrator_male.safetensors` with two decoding steps. Recognized races keep their configured race/gender voices; explicit voice and NPC overrides still take precedence. The narrator fallback is separate from the Human profiles, so you can change it independently. For example, `"npc_overrides": {"4949": "orc_male"}` assigns that NPC the Orc male profile. The JSON output includes `race`, `gender`, and `npc_id` when available.

`UnitRace` frequently returns nil for NPCs even when their gender and GUID are available. The addon first uses saved NPC race assignments, public API race information, and confirmed NPC-ID mappings. Zephras Citizen (254100) is mapped to Skyborne based on in-game confirmation. Unknown NPCs are probed with an invisible `PlayerModel`. Its display ID is matched against 15,444 race/gender records from VoiceOver, then known character FileDataIDs from the [WoW file list](https://github.com/wowdev/wow-listfile) provide a model fallback. The legacy display snapshot covers 21 races including Goblin; new or reassigned Forever displays may differ, and Skyborne still need confirmed NPC assignments. See [race data and attribution](docs/RACE_DATA.md) for coverage and reproducible generation. This uses documented [SetUnit](https://github.com/Gethe/wow-ui-source/blob/forever/Interface/AddOns/Blizzard_APIDocumentationGenerated/FrameAPICharacterModelBaseDocumentation.lua) and [GetModelFileID](https://github.com/Gethe/wow-ui-source/blob/forever/Interface/AddOns/Blizzard_APIDocumentationGenerated/SimpleModelAPIDocumentation.lua) APIs. Model loads get up to 600 ms before the text is sent; a newer message cancels publication of an older pending message.

A model identifies appearance, which may differ from lore race: an elf model alone cannot prove an NPC is Skyborne. Unknown models remain unassigned. Use `/fdb npc` to inspect the evidence, or `/fdb race Skyborne` (also `Goblin`, etc.) to save the correct identity for that NPC ID. This does not reclassify every NPC that shares its model. Reopen dialogue after assigning a race.

Ambient chat uses public sender GUIDs and confirmed NPC-ID mappings, exact matches to accessible units, or the recent identity cache; it does not guess from NPC names. Secret API values are skipped. `/fdb status` shows the last transmitted NPC's race and whether it came from the API, a lookup, an override, or model appearance.

Pocket speech reads the quest title and text without announcing the speaker's name. Long passages are split into short chunks, with synthesis ahead of playback. New dialogue immediately cancels playback and discards stale audio. Native generation checks cancellation between chunks and joins its workers before the next request.

### Local runtime and custom voices

PocketTTS.cpp and ONNX Runtime run inside the Go process. The app performs no network requests during synthesis. It loads and verifies the pinned April English models and restores existing voice states directly. `-native-dir` selects the model/preset folder; `-models-dir` optionally overrides its `models/` subfolder. `-cpu-threads` sets the native inference budget (default 1).

Python remains under `tools/voices/` only for optional custom-voice export. See [the voice export guide](docs/VOICE_CLONING.md). Profiles use presets or `.safetensors` paths; raw recordings must be exported before use. Base models, recordings, previews, and developer environments remain Git-ignored. Only configured custom voice states are packaged.

Decoded messages are printed as one JSON object per line on stdout. Discovery and speech diagnostics go to stderr. To save text without speech:

```powershell
.\foreverdubbed.exe -mute > dialogue.jsonl
```

The reader scans all monitors about once per second until it finds the square. It then samples only that region every 75 ms. Four failed reads trigger rediscovery, including after dragging, resizing, or moving the game window. Only one visible addon instance is supported at a time. Full desktop scans stay in memory; screenshots are neither saved nor uploaded.

For a diagnostic screenshot, explicitly run `.\foreverdubbed.exe -snapshot capture.png`. It waits three seconds, saves one PNG of the capture surface (the WoW window on macOS, the entire desktop on Windows), reports whether it found a valid tile, and exits. Keep WoW visible, use `/fdb unlock` so the square stays displayed, and move the pointer and other windows away from it. The PNG can be inspected locally or passed to `-image capture.png`. Windows desktop snapshots include other visible windows; macOS snapshots contain only the selected game window. The normal reader never saves images.

## Troubleshooting

- **No square:** `/fdb on`, `/fdb reset`, then `/fdb test`. Check `/fdb status` and that the addon appears in WoW's AddOns list. `/console scriptErrors 1` enables Lua error reporting.
- **Square visible but not found:** check that both the addon and companion were updated, and `/fdb status` reports addon 0.5.0. Try `/fdb unlock` and `/fdb cell 2`, move the mouse away, and ensure the entire border is visible. Test in borderless/windowed mode. The reader reports whether there was no finder pattern or whether a candidate failed its palette, border, or checksum checks. If needed, try `/fdb cell 3` or save a diagnostic PNG. The dark palette is calibrated from the screen, but its closely spaced colors have less noise tolerance: HDR/color filters, dark-level compression, blur, or clipped colors can prevent reading. Use lossless, unscaled captures. The decoder supports integer cell sizes of 2–8 desktop pixels; externally scaling or compressing the game image can prevent decoding.
- **Found but no speech:** run the companion from a terminal to see native engine errors, and test with `-speak-test`. Ensure `-mute` is absent and check Windows' default audio output. `-tts sapi` bypasses Pocket TTS for comparison.
- **Native runtime missing:** extract the complete release with ONNX Runtime DLLs beside the executable. Source builds require `go run ./tools/native`, followed by `go run ./tools/build`. Windows may need the Microsoft Visual C++ x64 redistributable for ONNX Runtime.
- **Missing/checksum-mismatched model or preset:** run `go run ./tools/models` in a checkout. Custom states must exist at their configured paths. Model files from another checkpoint are not interchangeable.
- **Square disappears:** normal after the transmission timeout; `/fdb test` sends again. `/fdb unlock` keeps it displayed during setup.
- **Long text takes time:** each page carries 1,056 bytes and lasts 250 ms. A 2 KB message needs two pages, or 0.5 seconds per cycle, plus discovery and TTS startup. Missed pages are recovered on later cycles. Text is spoken only after every page passes validation.
- **Changing quest screens interrupts narration:** intentional latest-message behavior. Ambient NPC speech can also interrupt; toggle it with `/fdb chat`.

The border includes a reference swatch for every palette color. The decoder measures these on every captured page and compares data cells to the observed colors, adapting to uniform gamma and color changes without assuming exact screen RGB values. Ambiguous or insufficiently separated colors are rejected. This improves tolerance; it does not make the transport immune to all display processing.

There is no forward error correction. Per-page and whole-message Adler-32 checksums reject most accidental damage, and repeated pages provide retries. Checksums are for accidental corruption, not authentication. The companion accepts a valid tile anywhere visible on the desktop.

## Build and test

The Go code requires Go 1.22+. Building embedded PocketTTS additionally requires CMake, Git, and C/C++17 compilers. Runtime dependencies are native libraries and data files, not Python. [Native build documentation](native/README.md) describes pinned dependencies, model checksums, and export compatibility.

```sh
go test ./...
go vet ./...
go run ./tools/native
go run ./tools/models
go run ./tools/build
```

`tools/build` defaults to the Windows/amd64 release; `-target darwin -arch arm64|amd64` produces macOS releases (see [the Linux/macOS build guide](docs/macos.md)). Both it and `tools/native` automatically select the installed MinGW-w64 cross-compilers on Ubuntu/WSL; explicit `CC`/`CXX` values override this selection. `tools/native` prepares matching dependencies in `.runtime/sdk/windows_amd64` and DLLs in `.runtime/native`. For an alternate runtime directory, use `-out <directory>` with setup/model downloads and `-native-dir <directory>` with the builder. It rejects missing or wrong-architecture libraries. `scripts/build.ps1` delegates to this Go tool.

Real native speech checks (no game or audio device required):

```sh
go run ./tools/native -target host
go run -tags pocket_native ./tools/voicecheck
go run -tags pocket_native ./tools/voicecheck -voice undead_male
```

Speech commands require `CGO_ENABLED=1`, C/C++ compilers, and ONNX Runtime on the OS library search path (see [native instructions](native/README.md)); live desktop capture/playback are not implemented there. The model-free Go tests run without native libraries. Python export-tool tests are optional: `python -m unittest discover -s tools/voices -p 'test_*.py'`. Lua 5.1+ enables additional addon compatibility tests through `LUA=/path/to/lua`.

The portable decoder can read PNGs on any OS. For a paged message, supply one unmodified screenshot of each distinct page, in any order:

```sh
go run ./cmd/foreverdubbed -image page1.png,page2.png,page3.png
```

Tests cover voice routing and overrides, cancellation, native voice selection and errors, streamed PCM validation and bounded playback, speaker metadata, the Lua/Go byte and palette contract, Unicode spanning pages, out-of-order/duplicate pages, session changes, sequence wraparound, invalid dimensions, corruption, gamma/tint/noise transforms, damaged reference swatches, moved tiles, negative monitor coordinates, missing NPC races, display lookup precedence, model-load timing and stale identities, saved race assignments, settings migration, physical pixel sizing at multiple resolutions/UI scales, and all supported cell sizes. Lua tests use mocked game APIs; they do not substitute for testing the real client. Tests explicitly skip the Lua checks if an interpreter is unavailable.

See [PROTOCOL.md](PROTOCOL.md) for the wire format. API references used: [Forever gossip API source](https://github.com/Gethe/wow-ui-source/blob/forever/Interface/AddOns/Blizzard_APIDocumentationGenerated/GossipInfoDocumentation.lua), [Forever quest UI source](https://github.com/Gethe/wow-ui-source/blob/forever/Interface/AddOns/Blizzard_UIPanels_Game/Mainline/QuestFrame.lua), [Windows BitBlt](https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-bitblt), and [SpeechSynthesizer.Speak](https://learn.microsoft.com/en-us/dotnet/api/system.speech.synthesis.speechsynthesizer.speak?view=netframework-4.8.1).
