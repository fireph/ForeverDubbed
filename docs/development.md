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

Speech commands require `CGO_ENABLED=1`, C/C++ compilers, and ONNX Runtime on the OS library search path (see [native instructions](../native/README.md)); live capture/playback are not implemented on Linux. The model-free Go tests run without native libraries. Python export-tool tests are optional: `uv run --project tools/voices --locked python -m unittest discover -s tools/voices -p 'test_*.py'` (see [voice tool setup](../tools/voices/README.md)). Lua 5.1+ enables additional addon compatibility tests through `LUA=/path/to/lua`.

The portable decoder can read PNGs on any OS. For a paged message, supply one unmodified screenshot of each distinct page, in any order:

```sh
go run ./cmd/foreverdubbed -image page1.png,page2.png,page3.png
```

The default suite covers protocol/Lua compatibility, identity and voice routing, speech cancellation and queueing, fake capture/audio devices, update rollback, and release packaging. Model-free C++ speech tests compile with `c++` and skip if it is unavailable. Lua tests also skip without an interpreter and use mocked game APIs, so they do not substitute for testing the real client.

## Code organization

- `cmd/foreverdubbed`: startup/options in `main.go`, optical polling in `capture.go`, speech queue policy in `speech.go`, and offline image/snapshot commands in `images.go`.
- `internal/platform`: native capture, audio devices, and system voices. OS-specific files stay in this package with `_windows`/`_darwin` suffixes and cgo build constraints. `capture_window.go`, `pcm.go`, and `playback_status.go` hold shared policy; fake-device tests run on any host.
- `internal/desktop` and `internal/update`: platform-specific window and process integration stays with the feature that uses it, selected by filename suffixes/build constraints.
- `internal/pocket`: the Go speech engine and C ABI; `bridge.cpp` owns streaming and worker shutdown. `pocket_tts.hpp` contains the complete locally adapted PocketTTS implementation, including text preparation, generation stopping, sentence fades, and saved voice tensor validation/import. Model-free C++ tests include this same header with `POCKET_TTS_HELPERS_ONLY` to omit inference SDK dependencies.
- `tools/build`: release orchestration, manifests, file selection, and ZIP output, with separate `macos_*` and `windows_*` files for target packaging. These names use OS prefixes rather than reserved suffixes because both targets must compile on every build host.

Use `gofmt` for Go and the root `.clang-format` for maintained C++/Objective-C bridges. Keep formatting-only changes out of the pinned upstream `pocket_tts.hpp` so local inference fixes remain easy to compare with upstream:

```sh
gofmt -w cmd internal tools
clang-format -i internal/pocket/bridge.cpp internal/pocket/bridge.h
clang-format -i internal/platform/*.cpp internal/platform/*.h internal/platform/*.m internal/desktop/*.m
go test -race ./...
go test -race -tags "gui ci" ./internal/desktop ./cmd/foreverdubbed
```

Tests should exercise observable contracts: window ownership, audio ordering and cleanup, queue controls, decoding, safe update replacement, and packaged file contents. Native live-capture tests remain opt-in because they require a running game and OS permissions. Set `FDB_TEST_CAPTURE_APP` and run `go test ./internal/platform` on Windows or macOS with cgo enabled. Real speech recovery/fade tests use `FDB_TEST_NATIVE_DIR` and `-tags pocket_native` as described in the native guide.

When fixing a bug, first reproduce it with a focused regression test. Check the resulting audio, file contents, or state rather than implementation details, and extend an existing test when it already exercises the affected behavior.

## Desktop interface

macOS release builds produce a self-contained `ForeverDubbed.app`; Windows builds produce `foreverdubbed.exe`. Release builds include the Fyne 2.8.1 GUI via `-tags gui,pocket_native`. A plain `go build ./cmd/foreverdubbed` remains terminal-only; add `-tags gui` for the interface without PocketTTS, then run with `-tts system` or `-mute`. `-headless` also bypasses the interface in a GUI build.

GUI tests use Fyne's `ci` software driver, so Linux CI needs no display server or graphics development libraries. To preview the actual desktop interface on Linux, install Fyne's X11/OpenGL development prerequisites; live game capture and playback still require Windows or macOS. Status updates are shared through `internal/appstate` and applied on Fyne's event loop. PCM playback status starts after the first successful device queue operation and remains active until playback finishes or is canceled.

For runtime validation, check startup errors, game absent/present, tile found/lost, preparing/playing/idle speech, minimizing or closing to the tray, restoring from the tray, and quitting during playback. Unit tests and cross-compilation cannot validate native system-tray behavior.

### Logo and application icons

The original artwork lives in `internal/appicon/assets/`. `forever-dubbed-banner.png` is embedded for the centered desktop header. `forever-dubbed-logo.png` supplies the window and tray icons and is resized into the macOS bundle's `app.icns` during packaging. Images retain their transparent background.

Windows executables include the ICO through `internal/appicon/icon_windows_amd64.syso`. Keep this generated resource in source control so direct `go build` commands also include the icon. After replacing the ICO, regenerate it with MinGW-w64 installed:

```sh
go generate ./internal/appicon
```

On a Windows MinGW installation whose resource compiler is named `windres`, run `windres --input assets/icon.rc --output icon_windows_amd64.syso --output-format coff --target pe-x86-64` from `internal/appicon/` instead. Rebuild the executable or macOS bundle after changing the artwork.

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

## Automatic updates

`internal/update` uses the public `fireph/ForeverDubbed` latest-release API with a 15-second startup-check deadline. It only offers newer stable semantic versions, selects the exact OS/architecture asset, and requires `SHA256SUMS.txt`. Only the current names (`ForeverDubbed-windows-<arch>-portable.zip` and `ForeverDubbed-mac-<arch>.zip`) are accepted. Windows uses the portable payload for both installed and portable updates; it does not change installer registration or shortcuts. macOS verifies the new bundle against the installed bundle's designated code-signing requirement.

The builder includes a standalone GUI updater without PocketTTS dependencies and `release-manifest.json`. The desktop downloads/verifies/extracts before changing any installed files. It copies the existing updater into a private staging folder, waits for readiness, then shuts down normally. The updater waits for the parent process to exit before moving locked files, reports installation progress, rolls back on replacement errors, and relaunches with the original arguments and working directory. Unrelated files and Fyne preferences are untouched; bundled voice/config files are replaced. Successful jobs are cleaned after the helper exits; failures retain logs and backups. Updates require sufficient space for the download, staged payload, and previous files. Manual release installation remains available.

The only local product version is `## Version:` in `addon/ForeverDubbed/ForeverDubbed.toc`. `internal/buildinfo` embeds that metadata for ordinary Go builds, and addon Lua reads it through WoW's addon metadata API. Tagged GitHub builds override it with `vMAJOR.MINOR.PATCH` from the Git tag: the desktop, updater, macOS app metadata, Windows installer, release manifest, and packaged addon all receive that version. The builder stamps a copy under `dist/addon/ForeverDubbed`, leaving the source TOC unchanged. Both the standalone addon ZIP and desktop packages use that same staged copy. For local releases, change only the TOC version and rebuild; for tagged releases, choose the version in the tag. Windows auto-updates also refresh Installed apps' version for the matching registered installation. Publish both the macOS ZIP and DMG and include the ZIP in release checksums. A machine running an older release without an updater must install this feature manually once.

Archive and manifest paths are validated using the same rules on every build host. Parent directories participate in case-collision checks, so `Data/a` and `data/b` are rejected even on Linux. File/directory conflicts and manifests listing themselves are rejected before installation.

Tests use fake HTTP transports and temporary installations to cover version/asset selection, download verification and cancellation, archive traversal/link rejection, file replacement, rollback, and confirmation/progress UI. Run `go test ./internal/update ./tools/build` and `go test -tags "gui ci" ./internal/desktop`. Real macOS signature checks and full native GUI update/restart need validation on their respective OSes.
