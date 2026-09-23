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

Tests cover voice routing and overrides, cancellation, native voice selection and errors, streamed PCM validation and bounded playback, speaker metadata, the Lua/Go byte and palette contract, Unicode spanning pages, out-of-order/duplicate pages, session changes, sequence wraparound, invalid dimensions, corruption, gamma/tint/noise transforms, damaged reference swatches, moved tiles, negative monitor coordinates, missing NPC races, display lookup precedence, model-load timing and stale identities, saved race assignments, settings migration, physical pixel sizing at multiple resolutions/UI scales, and all supported cell sizes. Lua tests use mocked game APIs; they do not substitute for testing the real client. Tests explicitly skip the Lua checks if an interpreter is unavailable.


## Desktop interface

macOS release builds produce a self-contained `ForeverDubbed.app`; Windows builds produce `foreverdubbed.exe`. Release builds include the Fyne 2.8.1 GUI via `-tags gui,pocket_native`. A plain `go build ./cmd/foreverdubbed` remains terminal-only; add `-tags gui` for the interface without PocketTTS, then run with `-tts system` or `-mute`. `-headless` also bypasses the interface in a GUI build.

GUI tests use Fyne's `ci` software driver, so Linux CI needs no display server or graphics development libraries. To preview the actual desktop interface on Linux, install Fyne's X11/OpenGL development prerequisites; live game capture and playback still require Windows or macOS. Status updates are shared through `internal/appstate` and applied on Fyne's event loop. PCM playback status starts after the first successful device queue operation and remains active until playback finishes or is canceled.

For runtime validation, check startup errors, game absent/present, tile found/lost, preparing/playing/idle speech, minimizing or closing to the tray, restoring from the tray, and quitting during playback. Unit tests and cross-compilation cannot validate native system-tray behavior.
