# macOS companion

The macOS companion targets macOS 14 Sonoma or newer, on Apple Silicon (`arm64`) or Intel (`amd64`). It uses ScreenCaptureKit for the optical tile and AudioQueue for streamed PocketTTS audio. `-tts system` uses the installed macOS voices through `say`. Windows continues to support PocketTTS and `-tts sapi` (also available as `-tts system`).

## Build macOS releases on Linux

The Go build tools run on Linux. OSXCross supplies the macOS C/C++/Objective-C compiler, linker, and Apple SDK for the cgo parts. Setting only `GOOS=darwin` is insufficient because both screen capture and PocketTTS use native libraries.

On Ubuntu 24.04 or newer, install Go 1.22+ and these prerequisites:

```sh
sudo apt-get update
sudo apt-get install -y clang llvm lld cmake build-essential curl git xz-utils bzip2 cpio
bash scripts/setup-macos-cross.sh
```

Setup pins an [OSXCross](https://github.com/tpoechtrager/osxcross) revision and downloads the [macOS 14.5 SDK archive](https://github.com/joseluisq/macosx-sdks/releases/tag/14.5), verifying its SHA-256. Everything is installed under `.runtime/osxcross`; the setup does not change your system compiler. You can also supply an existing OSXCross installation as the second argument to the environment script below.

From the repository root, in Bash:

```sh
source scripts/macos-cross-env.sh arm64
go run ./tools/native -target darwin -arch arm64 -out .runtime/native-darwin-arm64
go run ./tools/models -out .runtime/native-darwin-arm64
go run ./tools/build -target darwin -arch arm64 -native-dir .runtime/native-darwin-arm64
```

The result is `dist/ForeverDubbed-darwin-arm64.zip`, including the executable, ONNX Runtime dylibs, models, voice presets, and addon. Use `amd64` in all four commands for Intel. Each build also writes `dist/foreverdubbed`; copy or extract the ZIP to keep separate architecture builds. Keep target dependency directories separate from the Windows runtime.

Do **not** export `GOOS` or `GOARCH` for these `go run` commands: they execute the setup/packaging tools on Linux, and the tools select the child build target. `CC` and `CXX` from the environment script select the same compiler for CMake and Go. `MACOS_SDK` selects CMake's SDK; the deployment target is macOS 14.0. The environment script disables cgo for the Linux helper programs, and the packager enables it for the macOS child build.

For a smaller capture/system-voice build without PocketTTS models:

```sh
source scripts/macos-cross-env.sh arm64
GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -o dist/foreverdubbed ./cmd/foreverdubbed
```

Run that binary with `-tts system` or `-mute`. The default PocketTTS backend requires the full native build above.

## GitHub Actions

[The macOS workflow](../.github/workflows/macos.yml) builds both architectures on `ubuntu-24.04`, runs the portable Go tests/vet, caches the pinned OSXCross toolchain, and uploads the two release ZIPs. It runs for pull requests, pushes to `main`, and manual dispatch. It does not publish releases or run macOS executables on Linux. Native screen/audio behavior must be checked on a Mac.

## Build directly on a Mac

Install Go 1.22+, CMake 3.28+, Git, and Xcode command-line tools with a macOS 14+ SDK. From the repository root:

```sh
xcode-select --install
go run ./tools/native -target darwin -out .runtime/native-darwin
go run ./tools/models -out .runtime/native-darwin
go run ./tools/build -target darwin -native-dir .runtime/native-darwin
```

The default architecture matches the Mac. The resulting release ZIP has the same layout as a Linux cross-build.

## Run and grant capture permission

Extract the whole ZIP, then open Terminal in the extracted directory:

```sh
./foreverdubbed -tts system -speak-test 'ForeverDubbed is ready.'
./foreverdubbed -mute
```

On the first capture attempt, allow **Screen Recording** (called **Screen & System Audio Recording** on some versions) for your terminal or ForeverDubbed under **System Settings → Privacy & Security**. Quit and reopen the terminal/application after granting access, then run the command again. Keep WoW visible and use `/fdb unlock` to display the tile. The app reports decoded dialogue as JSON. To enable PocketTTS, run `./foreverdubbed` without `-mute`.

Useful checks:

```sh
./foreverdubbed -tts system -voices
./foreverdubbed -tts system
./foreverdubbed -speak-test 'Testing PocketTTS playback.'
./foreverdubbed -snapshot capture.png
./foreverdubbed -image capture.png
```

Capture searches every display at its native pixel scale. For mixed Retina/non-Retina displays, screenshots arrange the displays side by side; logged tile coordinates refer to that combined image, not macOS desktop points. Keep the whole tile on one display. Restart after connecting/disconnecting a monitor or changing display resolution. Screenshots include the other visible windows and are saved only when explicitly requested with `-snapshot`.

Before distributing a build, check on a Mac that a tile decodes at the top and bottom of each display, tracked-region decoding matches the initial full-screen scan, long speech finishes without losing its ending, and new dialogue/Ctrl+C interrupts playback. Linux compilation and tests cannot verify Screen Recording permission, Retina capture, or speaker output.
