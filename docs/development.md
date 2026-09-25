# Development and testing

The Go code requires Go 1.27.1+. Building embedded PocketTTS additionally requires CMake, Git, and C/C++17 compilers. Runtime dependencies are native libraries and data files, not Python. [Native build documentation](../native/README.md) describes pinned dependencies, model checksums, and export compatibility.

```sh
go test ./...
go vet ./...
go test -tags "gui ci" ./internal/desktop ./cmd/foreverdubbed
go run ./tools/native
go run ./tools/models
go run ./tools/build
```

`tools/build` defaults to the Windows/amd64 release; `-target darwin -arch arm64|amd64` produces macOS releases (see [the Linux/macOS build guide](macos.md)). Both it and `tools/native` automatically select the installed MinGW-w64 cross-compilers on Ubuntu/WSL; explicit `CC`/`CXX` values override this selection. `tools/native` prepares matching dependencies in `.runtime/sdk/windows_amd64` and DLLs in `.runtime/native`. For an alternate runtime directory, use `-out <directory>` with setup/model downloads and `-native-dir <directory>` with the builder. It rejects missing or wrong-architecture libraries. `scripts/build.ps1` delegates to this Go tool.

Windows archives are named `ForeverDubbed-windows-amd64-portable.zip`; macOS archives use `ForeverDubbed-mac-<arch>.zip` (the compiler target remains `darwin`). To also create `ForeverDubbed-windows-amd64-setup.exe`, install [NSIS](https://nsis.sourceforge.io/Docs/) (`sudo apt-get install nsis` on Ubuntu) and run `go run ./tools/build -windows-installer`. GitHub Actions enables this flag. The installer uses the portable package's exact file manifest, installs under `%LOCALAPPDATA%\Programs\ForeverDubbed` by default without elevation, creates a Start menu shortcut, and registers an uninstaller. Quit the running app before upgrading; packaged files are replaced, so back up any edits to bundled configuration or voices first. Uninstall removes packaged files and empty directories, leaving additional user files and desktop preferences intact. The addon must still be copied into WoW separately.

Real native speech checks (no game or audio device required):

```sh
go run ./tools/native -target host
go run -tags pocket_native ./tools/voicecheck
go run -tags pocket_native ./tools/voicecheck -voice undead_male
```

Speech commands require `CGO_ENABLED=1`, C/C++ compilers, and ONNX Runtime on the OS library search path (see [native instructions](../native/README.md)); live capture/playback are not implemented on Linux. The model-free Go tests run without native libraries. Python export-tool tests are optional: `python -m unittest discover -s tools/voices -p 'test_*.py'`. Lua 5.1+ enables additional addon compatibility tests through `LUA=/path/to/lua`.

The portable decoder can read PNGs on any OS. For a paged message, supply one unmodified screenshot of each distinct page, in any order:

```sh
go run ./cmd/foreverdubbed -image page1.png,page2.png,page3.png
```

Tests cover voice routing and overrides, cancellation, native voice selection and errors, streamed PCM validation and bounded playback, speaker metadata, the Lua/Go byte and palette contract, Unicode spanning pages, out-of-order/duplicate pages, session changes, sequence wraparound, invalid dimensions, corruption, gamma/tint/noise transforms, damaged reference swatches, moved tiles, negative monitor coordinates, desktop race-layer precedence, unchanged VoiceOver data, raw display/model observations, model-load timing and stale identities, saved race assignments and clearing, settings migration, physical pixel sizing at multiple resolutions/UI scales, and all supported cell sizes. Lua tests use mocked game APIs; they do not substitute for testing the real client. Tests explicitly skip the Lua checks if an interpreter is unavailable.


## Desktop interface

macOS release builds produce a self-contained `ForeverDubbed.app`; Windows builds produce `foreverdubbed.exe`. Release builds include the Fyne 2.8.1 GUI via `-tags gui,pocket_native`. A plain `go build ./cmd/foreverdubbed` remains terminal-only; add `-tags gui` for the interface without PocketTTS, then run with `-tts system` or `-mute`. `-headless` also bypasses the interface in a GUI build.

GUI tests use Fyne's `ci` software driver, so Linux CI needs no display server or graphics development libraries. To preview the actual desktop interface on Linux, install Fyne's X11/OpenGL development prerequisites; live game capture and playback still require Windows or macOS. Status updates are shared through `internal/appstate` and applied on Fyne's event loop. PCM playback status starts after the first successful device queue operation and remains active until playback finishes or is canceled.

For runtime validation, check startup errors, game absent/present, tile found/lost, preparing/playing/idle speech, minimizing or closing to the tray, restoring from the tray, and quitting during playback. Unit tests and cross-compilation cannot validate native system-tray behavior.

### Quest-dialog styling

The desktop UI uses a parchment reading area, textured dark frame, gold headings, and red beveled controls inspired by the classic WoW quest dialog. The frame and paper are drawn locally and scale with the window; they do not use extracted game textures. The bundled regular, bold, and italic [Caudex fonts](https://github.com/google/fonts/tree/main/ofl/caudex) provide consistent serif typography on both platforms. Their [SIL Open Font License](licenses/Caudex-OFL.txt) ships with the release and inside the macOS app's `Contents/Resources/licenses` directory. Theme and decorative drawing code live in `internal/desktop/quest_theme.go`; state and interaction behavior remain in the existing dashboard and app controller.


## Addon playback controls

FDB5 flags describe NUL-separated UTF-8 fields: flags 0 has speaker/title/text;
flags 1 adds race/gender/NPC ID; flags 2 adds display ID/model ID/race override;
flags 3 adds quest objectives as a tenth field. Objectives are never appended
to the main text by the addon. Empty optional metadata fields still occupy their
positions when a later field is present. Flags 0/1/2 remain readable, but an old
addon that embeds objectives in text must be updated for dialogue-only speech.
The desktop keeps received fields intact and chooses spoken quest content when
playback starts. Separate title and objectives options both default off.

FDB5 kinds 7 (Stop) and 8 (Skip) carry three empty string fields and no voice
metadata. They share the dialogue session/sequence counter and assembler
deduplication, so repeated captures execute a command once. The speech worker
handles controls before queueing or synthesis; both cancel the current utterance
and let the existing queue advance, matching the desktop controls. A control
does not replace the displayed last-dialogue metadata.

Control frames replace older dialogue immediately, cancel unresolved speaker
lookups, and stay on screen for up to 15 seconds. For the first 1.5 seconds,
new dialogue is deferred (only the latest is retained). This gives the default
one-second discovery scan a chance to receive the command. There is no
acknowledgment channel: obscured tiles, unusually slow scanning, and commands
overwritten by later commands can still prevent delivery.

WoW loads the addon's root `Bindings.xml` automatically. `Controls.lua` supplies
the binding labels/functions and minimap button; its saved angle lives in
`ForeverDubbedDB.minimapAngle`. Tests cover click routing, drag suppression and
persistence, control priority, late identity callbacks, and duplicate frames.
