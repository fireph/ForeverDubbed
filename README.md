# ForeverDubbed

A WoW Forever addon and Go companion that read NPC and quest dialogue aloud. The addon draws a small RGB data square; the Go app finds it inside the WoW game window on Windows and macOS, decodes the text, and speaks using **Pocket TTS on your CPU**. The Go companion captures, decodes, selects voices, and streams audio through an in-process PocketTTS.cpp engine. No Python interpreter or local HTTP service is used at runtime. Speech stays on your computer. No OCR or game-memory access is used. Windows SAPI and macOS system voices remain available as fallbacks.

The square uses **16 subtle dark-navy calibrated colors** and a **48 × 48 data grid**, with a one-cell calibration ring and a **light-blue 2px outer outline**. At the default **2 × 2 pixel cell size**, it occupies **104 × 104 physical pixels** and carries **1,056 bytes per page**. The outline stays 2 physical pixels thick at every cell size. A crisp light-blue sine wave with a 2-cell stroke measured perpendicular to the curve drifts left through the middle half of the data area, animating at 15 fps and completing a loop every 3.2 seconds. Its hard-coded shape moves one whole cell per frame with edge wrapping, so the stroke never changes shape during motion. Both encoder and decoder skip its cells. This FDB5 format requires updating both the addon and companion to 0.6.0. You can enlarge cells with `/fdb cell 3` (154 × 154 pixels) if your display needs more sampling margin; the capacity stays the same. This is a custom optical format, not a standard QR code.

**Native runtime migration:** rebuild the companion and bundle the native runtime/model files below. Existing April-model `.safetensors` voices are preserved. The old Python service and setup scripts are no longer used.

See [CHANGELOG.md](CHANGELOG.md) for earlier releases and [PROTOCOL.md](PROTOCOL.md) for the optical format.

## Quick start on macOS

The macOS release requires macOS 14+ on Apple Silicon. Download `ForeverDubbed-mac-arm64.dmg` and open the disk image.

Drag **ForeverDubbed.app** to Applications and double-click it. The app includes its speech libraries, models, and voices; no Terminal, Go, or compiler tools are needed. The addon is included separately in the disk image and as `ForeverDubbed-addon.zip` on the release.

Grant **Screen Recording** permission under **System Settings → Privacy & Security**, then quit and reopen ForeverDubbed if needed. Capture is limited to the **World of Warcraft Beta.app** game window; it waits when the game is unavailable and never falls back to the desktop.

Install the addon and test it using the steps below. See the [macOS guide](docs/macos.md) for capture troubleshooting and building on Linux, GitHub Actions, or a Mac.

## Quick start on Windows

Windows capture targets **WoWB.exe** only, using Windows Graphics Capture (Windows 10 version 1903+ or Windows 11). It never falls back to desktop capture. See [Windows window-capture details](docs/windows.md) for executable selection, troubleshooting, and live checks.

Run `ForeverDubbed-windows-amd64-setup.exe` to install for your Windows user, with a Start menu shortcut and an entry in Installed apps. The addon is included under the installation folder and still needs copying into WoW (see below).

For a portable installation, download `ForeverDubbed-windows-amd64-portable.zip`, extract the entire folder and run `foreverdubbed.exe` (or the optional `Start-ForeverDubbed.cmd`). Keep the ONNX Runtime DLLs, `native/`, and `tts/` beside the executable. Python, uv, and Go are not needed to run a prepared release.

For source builds on Windows or Linux/WSL, see the [Windows build guide](native/README.md#build).

## Automatic updates

Installed macOS apps and installed or extracted Windows releases check [GitHub releases](https://github.com/fireph/ForeverDubbed/releases) in the background each time the desktop app starts. A newer stable version shows **OK** and **Later**. **OK** downloads the update with progress, verifies its SHA-256 checksum, then closes the app, installs the update, and restarts it. You can cancel during download/preparation. Offline or failed checks leave the app running normally; headless and source-checkout runs do not check for updates.

Updates replace bundled files, including `tts/voices.json` and voice states. Desktop preferences remain in your user settings. Windows updates preserve the installation location; macOS updates replace the app bundle and require the same signing identity. The app must be in a writable location (copy it out of the macOS disk image first). The separate updater keeps showing progress while the main app is closed and restores replaced files if installation fails. Failed jobs keep recovery files and `updater.log` in a `.foreverdubbed-update-*` folder in the Windows installation folder or beside the macOS app.

The WoW addon is included in release downloads, but updating the desktop does not install it into WoW. Continue copying addon updates into `Interface/AddOns` separately. Existing versions without the updater need one manual upgrade to an update-capable release.

## Desktop companion

The Fyne interface is styled like a classic WoW quest dialog, with parchment, a dark frame, serif text, and red-and-gold buttons. It shows whether the game window is available, whether the addon tile is connected, and whether speech is preparing, playing, idle, or muted. The **Stop** button beneath Audio interrupts the current speech without stopping game detection. It is enabled only while audio is playing.

Under **Read aloud**, choose which dialogue receives speech:

- **Quest dialogue:** main text of quest offers, progress, and completion.
- **NPC conversations:** non-quest NPC dialogue windows and quest-giver greetings.
- **NPC speech:** ambient NPC/boss say, yell, whisper, and emote text.

All three default to enabled and are saved across launches on Windows and macOS.
**Quest title** optionally reads the title before the main text, and **Quest objectives**
reads objectives after it. Both default to off and are saved across launches. Changes apply
when each quest starts speaking. Update both addon and desktop to 0.6.5, then
`/reload` WoW: the addon now sends the title, dialogue, and objectives separately.
Unchecking a category stops its current speech and removes its waiting messages.
Connection tests, books/item text, and Stop/Skip commands remain available.
The addon must also have NPC chat enabled (`/fdb chat`) to transmit ambient speech.

Enable **Queue new dialogue** to play incoming messages in order. Leave it unchecked
(the default) to interrupt speech and play the newest message. The app remembers
this setting. Unchecking it with messages waiting interrupts the current speech
and plays only the newest queued message. **Stop** skips the current speech;
in queue mode, the next waiting message then plays. Queue mode also shows a
**Skip** button: it advances to the next message, or stops if nothing else is
queued. Skip is enabled only during playback.

Close or minimize the window, or click **Minimize to tray**, to keep the companion running in the Windows system tray or macOS menu bar. Choose **Show ForeverDubbed** from its tray icon to reopen it. Choose **Quit** to stop capture and audio and exit.

For terminal-only operation, use `-headless`. One-shot commands such as `-voices`, `-version`, `-speak-test`, `-snapshot`, and `-image` run without opening the GUI. To save decoded dialogue, run with `-headless -mute` and redirect stdout to a file.

## Set up the addon

On either platform:

1. From the release, copy `addon/ForeverDubbed` into the Forever client's `Interface\AddOns` directory and enable it in the game.
2. Run the companion. It loads the native engine directly; there is no separate service to start.
3. Enter `/fdb test`. You should see the tile detected and hear the connection test.
4. Use `/fdb unlock`, move the tile, then `/fdb lock`. Prefer windowed or borderless game mode.

The release contains one application executable plus ONNX Runtime libraries, ONNX models, and `.safetensors` voices. It is not a single statically linked binary. Game-window capture/playback support Windows 10 version 1903+ and macOS 14+; the native voice-test tool also runs on Linux.

### Streaming speech

Streaming is always enabled. The decoder generates audio in batches of up to 1.2 seconds, including the first batch; shorter sentence endings are flushed immediately. Playback starts when the first batch is ready, so startup delay depends on generation speed. Audio is queued in at most 100 ms buffers for responsive cancellation. New dialogue stops playback and cancels native generation. The model is reused after its workers finish, preserving safe voice changes.

See [native speech validation](native/README.md#validate-real-voices) for generating sample clips and measuring first-audio latency.

## What it reads

- NPC gossip and quest greetings.
- Quest offers, including objectives; progress text; completion/reward text.
- NPC say, yell, whisper, and emote events, including boss dialogue.
- Item/book text when its current page becomes available.

It captures text exposed through the game API, not arbitrary text drawn by other addons. Merely selecting a quest in the quest log, cinematic subtitles, gossip option buttons, and UI text that the beta restricts are outside this version's coverage. Secret API values are skipped.

Each new message replaces the previous transmission. Rapidly clicking through dialogue or overlapping NPC chat can therefore skip earlier passages; this version follows the latest message rather than maintaining a narration backlog. A fully decoded new message interrupts the current speech. `/fdb chat` disables ambient NPC chat if you only want interaction windows.

The addon keeps the wave at 15 fps and switches text pages every 250 ms. It caches the 48 wave layouts, reuses a page decoding buffer, and updates only textures whose colors changed, reducing Lua work and texture updates. Unused data cells contain a cached noise pattern so the texture fills the square even for short messages. The receiver ignores this padding when reading the text; padding remains compatible with FDB5. The extended NPC metadata requires the 0.6.0 companion.

The square remains visible for at least 15 seconds after text arrives, or three complete page cycles for very long text, then hides. Closing a quest window does not immediately hide it, allowing the reader to finish. While unlocked, it remains visible for positioning. Settings and position are stored through WoW SavedVariables when the client saves them.

## Addon controls

| Command | Behavior |
| --- | --- |
| `/fdb stop` / `/fdb skip` | Interrupt current audio; advance waiting dialogue in queue mode |
| `/fdb test` | Display and speak a connection test |
| `/fdb unlock` | Show a test square and allow dragging |
| `/fdb lock` | Save/use the position with mouse input disabled |
| `/fdb cell 2` | Set cell size to 2–8 physical pixels; default 2 |
| `/fdb reset` | Move back near the top-left corner |
| `/fdb chat` | Toggle ambient NPC/boss dialogue |
| `/fdb off` / `/fdb on` | Disable/enable automatic dialogue transmission |
| `/fdb status` | Print version, settings, and last transmitted NPC identity/source |
| `/fdb npc` | Inspect the dialog/target NPC: race, source, gender, NPC ID, display ID, and model file ID |
| `/fdb race Skyborne` | Save an NPC race override and capture available display/model evidence; reopen dialog to resend |
| `/fdb race clear` | Remove that NPC’s saved race override |

The minimap book button controls the companion: **left-click to Skip**,
**right-click to Stop**, and **drag** to move it around the minimap. Its position
is saved with the addon settings. In WoW's **Key Bindings** UI, find
**ForeverDubbed** and assign **Stop current audio** and **Skip current audio**;
neither has a default shortcut.

Both commands interrupt the current message. If queue mode has messages waiting,
the next message plays, matching the desktop controls. The addon cannot read
playback status, so these controls are always available. The companion must be
running and able to capture the data square; commands briefly show the square
even after it has hidden or automatic dialogue has been turned off.
Update **both** the addon and companion for these controls, then restart WoW
after installing the new keybindings file.

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

Selection order is `-voice`, `npc_overrides`, race/gender, then `default`. Each race has an explicit `unknown` gender fallback. Missing, empty, or unmapped races use the custom narrator for every gender, through the existing `narrator_male` profile ID. This profile uses `custom/narrator_male.safetensors` with two decoding steps. Recognized races keep their configured race/gender voices; explicit voice and NPC overrides still take precedence. The narrator fallback is separate from the Human profiles, so you can change it independently. For example, `"npc_overrides": {"4949": "orc_male"}` assigns that NPC the Orc male profile. The JSON output includes resolved `race`, `gender`, `race_source`, `npc_id`, `display_id`, `model_id`, and `race_override` when available.

The **desktop companion** resolves race using separate layers: `/fdb race` overrides, confirmed custom NPC mappings, public API race, the unchanged 15,444-entry VoiceOver display snapshot, then model appearance. `data/custom-races.json` includes 51 user-confirmed Skyborne NPC IDs and two inferred adult Skyborne model mappings. NPC assignments specify race only; public gender takes priority over display/model gender. The child observation is saved as evidence and an NPC assignment, without generalizing its model.

The addon sends NPC ID, public API race/gender, the model file ID, and any saved override. Normal dialogue never calls or waits for `GetDisplayInfo()`, which returns zero on the observed beta client. Explicit `/fdb npc` inspection and `/fdb race` evidence collection can try it; a successfully observed display ID may be reused for that same GUID and sent as optional metadata. Model data gets up to 600 ms to load; a newer message cancels publication of an older pending message. Ambient chat uses public sender GUIDs and exact unit matches or recent observations, never a shared NPC name.

Use `/fdb npc` to inspect raw observations and display availability. `/fdb race Skyborne` (also `Goblin`, etc.) saves a per-NPC override and supporting evidence; `/fdb race clear` removes it. Reopen dialogue to apply either change. Overrides remain in WoW SavedVariables and take priority on the desktop. `/fdb status` reports transmitted observations; desktop JSON reports the resolved race and source.

Edit the companion's `data/custom-races.json` and restart it to update the custom dataset without updating the addon, or select another file with `-race-config PATH`. The companion embeds a default copy when the external file is absent. Custom data does not modify the VoiceOver snapshot. See [race data and attribution](docs/RACE_DATA.md) for the schema, provenance, and reproducible import.

Quest speech reads the main text without announcing the speaker's name. Titles and objectives are included only when their desktop options are enabled; both default off. Long passages are split into short chunks, with synthesis ahead of playback. New dialogue immediately cancels playback and discards stale audio. Native generation checks cancellation between chunks and joins its workers before the next request.

### Local runtime and custom voices

PocketTTS.cpp and ONNX Runtime run inside the Go process. Speech synthesis makes no network requests. The desktop separately contacts GitHub at startup to check for updates; update files download only after confirmation. It loads and verifies the pinned April English models and restores existing voice states directly. `-native-dir` selects the model/preset folder; `-models-dir` optionally overrides its `models/` subfolder. `-cpu-threads` sets the native inference budget (default 1).

Python remains under `tools/voices/` only for optional custom-voice export. See [the voice export guide](docs/VOICE_CLONING.md). Profiles use presets or `.safetensors` paths; raw recordings must be exported before use. Base models, recordings, previews, and developer environments remain Git-ignored. Only configured custom voice states are packaged.

In headless mode, decoded messages are printed as one JSON object per line on stdout. Discovery and speech diagnostics go to stderr. To save text without speech:

```powershell
.\foreverdubbed.exe -headless -mute > dialogue.jsonl
```

The reader searches only the selected game window about once per second until it finds the square. It then decodes the tile region every 75 ms from game-window frames. Four failed reads trigger rediscovery, including after resizing or reopening the game. Windows selects `WoWB.exe`; macOS selects `World of Warcraft Beta.app`. If the game is unavailable, the reader waits; it never captures the desktop instead. Captures stay in memory; normal reading neither saves nor uploads screenshots.

For a diagnostic screenshot, explicitly run `.\foreverdubbed.exe -snapshot capture.png`. It waits three seconds, saves one PNG of the selected game window, reports whether it found a valid tile, and exits. Keep WoW visible, use `/fdb unlock` so the square stays displayed, and move the pointer and other windows away from it. The PNG can be inspected locally or passed to `-image capture.png`. On both platforms, snapshots contain only the selected game window. The normal reader never saves images.

## Troubleshooting

- **No square:** `/fdb on`, `/fdb reset`, then `/fdb test`. Check `/fdb status` and that the addon appears in WoW's AddOns list. `/console scriptErrors 1` enables Lua error reporting.
- **Square visible but not found:** check that both the addon and companion were updated, and `/fdb status` reports addon 0.6.0. Try `/fdb unlock` and `/fdb cell 2`, move the mouse away, and ensure the entire border is visible. Test in borderless/windowed mode. The reader reports whether there was no finder pattern or whether a candidate failed its palette, border, or checksum checks. If needed, try `/fdb cell 3` or save a diagnostic PNG. The dark palette is calibrated from the screen, but its closely spaced colors have less noise tolerance: HDR/color filters, dark-level compression, blur, or clipped colors can prevent reading. Use lossless, unscaled captures. The decoder supports integer cell sizes of 2–8 desktop pixels; externally scaling or compressing the game image can prevent decoding.
- **Found but no speech:** run the companion from a terminal to see native engine errors, and test with `-speak-test`. Ensure `-mute` is absent and check Windows' default audio output. `-tts sapi` bypasses Pocket TTS for comparison.
- **Native runtime missing:** extract the complete release with ONNX Runtime DLLs beside the executable. Source builds require `go run ./tools/native`, followed by `go run ./tools/build`. Windows may need the Microsoft Visual C++ x64 redistributable for ONNX Runtime.
- **Missing/checksum-mismatched model or preset:** run `go run ./tools/models` in a checkout. Custom states must exist at their configured paths. Model files from another checkpoint are not interchangeable.
- **Square disappears:** normal after the transmission timeout; `/fdb test` sends again. `/fdb unlock` keeps it displayed during setup.
- **Long text takes time:** each page carries 1,056 bytes and lasts 250 ms. A 2 KB message needs two pages, or 0.5 seconds per cycle, plus discovery and TTS startup. Missed pages are recovered on later cycles. Text is spoken only after every page passes validation.
- **Changing quest screens interrupts narration:** intentional latest-message behavior. Ambient NPC speech can also interrupt; toggle it with `/fdb chat`.

The tile uses the Chromaglyph OKLab palette centered at h 264, L 0.25, C 0.05, with minimum RGB distance √38 (about 6.16). The border includes a reference swatch for every palette color. The decoder measures these on every captured page and compares data cells to the observed colors, adapting to uniform gamma and color changes without assuming exact screen RGB values. Ambiguous or insufficiently separated colors are rejected. This improves tolerance; it does not make the transport immune to all display processing.

There is no forward error correction. Per-page and whole-message Adler-32 checksums reject most accidental damage, and repeated pages provide retries. Checksums are for accidental corruption, not authentication. The companion accepts a valid tile anywhere within the selected game window.

## Building from source and development

The local version is set only in `addon/ForeverDubbed/ForeverDubbed.toc` (`## Version:`). Tagged releases use the Git tag instead and stamp every packaged component automatically. Source builders can install the versioned addon from `dist/addon/ForeverDubbed` after building.

- [macOS builds](docs/macos.md): Linux cross-compilation, GitHub Actions, and native Mac builds.
- [Windows builds and native runtime](native/README.md): Windows and Linux/WSL toolchains, dependencies, and packaging.
- [Development and testing](docs/development.md): Go tests, speech validation, and decoder checks.

See [PROTOCOL.md](PROTOCOL.md) for the wire format. API references used: [Forever gossip API source](https://github.com/Gethe/wow-ui-source/blob/forever/Interface/AddOns/Blizzard_APIDocumentationGenerated/GossipInfoDocumentation.lua), [Forever quest UI source](https://github.com/Gethe/wow-ui-source/blob/forever/Interface/AddOns/Blizzard_UIPanels_Game/Mainline/QuestFrame.lua), [Windows Graphics Capture](https://learn.microsoft.com/en-us/windows/win32/api/windows.graphics.capture.interop/nf-windows-graphics-capture-interop-igraphicscaptureiteminterop-createforwindow), and [SpeechSynthesizer.Speak](https://learn.microsoft.com/en-us/dotnet/api/system.speech.synthesis.speechsynthesizer.speak?view=netframework-4.8.1).
