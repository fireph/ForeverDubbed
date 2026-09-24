# macOS companion

The macOS companion targets macOS 14 Sonoma or newer, on Apple Silicon (`arm64`) or Intel (`amd64`). It uses ScreenCaptureKit to capture only the game window for the optical tile and AudioQueue for streamed PocketTTS audio. `-tts system` uses the installed macOS voices through `say`. Windows continues to support PocketTTS and `-tts sapi` (also available as `-tts system`).

## Build macOS releases on Linux

The Go build tools run on Linux. OSXCross supplies the macOS C/C++/Objective-C compiler, linker, and Apple SDK for the cgo parts. Setting only `GOOS=darwin` is insufficient because both screen capture and PocketTTS use native libraries.

On Ubuntu 24.04, install Go 1.27.1+ and these prerequisites. Run these commands from the repository root in Bash:

```sh
sudo apt-get update
sudo apt-get install -y clang-18 llvm-18 lld-18 cmake build-essential curl git xz-utils bzip2 cpio
export PATH="$(llvm-config-18 --bindir):$PATH"
bash scripts/setup-macos-cross.sh
bash scripts/setup-macos-signing.sh
```

Repeat the `export PATH` command in a new shell before loading the cross-build environment. It makes the unversioned LLVM commands available and prevents `Missing ld64.lld` during setup. The setup and environment scripts also add `llvm-config --bindir` to `PATH`, since Ubuntu may expose `ld64.lld` only inside that directory (and as a versioned command in `/usr/bin`). The GitHub workflow installs matching LLVM 18 packages and keeps that directory on `PATH` across steps, including cache restores.

Setup pins an [OSXCross](https://github.com/tpoechtrager/osxcross) revision and downloads the [macOS 14.5 SDK archive](https://github.com/joseluisq/macosx-sdks/releases/tag/14.5), verifying its SHA-256. Everything is installed under `.runtime/osxcross`; the setup does not change your system compiler. You can also supply an existing OSXCross installation as the second argument to the environment script below.

From the repository root, in Bash:

```sh
source scripts/macos-cross-env.sh arm64
go run ./tools/native -target darwin -arch arm64 -out .runtime/native-darwin-arm64
go run ./tools/models -out .runtime/native-darwin-arm64
go run ./tools/build -target darwin -arch arm64 -native-dir .runtime/native-darwin-arm64
```

The release builder enables both the `gui` (Fyne 2.8.1) and `pocket_native` build tags. Fyne uses the macOS frameworks from the existing SDK; the Linux cross-build does not require Linux X11/OpenGL development packages.

The result is `dist/ForeverDubbed-darwin-arm64.zip`, containing `ForeverDubbed.app`, documentation, and the addon. The app contains its executable, ONNX Runtime dylibs, models, and voice presets. Use `amd64` in all four commands for Intel. Each build also writes `dist/ForeverDubbed.app` and `dist/foreverdubbed` for the selected architecture; copy or extract the ZIP to keep separate architecture builds. Keep target dependency directories separate from the Windows runtime.

Do **not** export `GOOS` or `GOARCH` for these `go run` commands: they execute the setup/packaging tools on Linux, and the tools select the child build target. `CC` and `CXX` from the environment script select the same compiler for CMake and Go. `MACOS_SDK` selects CMake's SDK; the deployment target is macOS 14.0. The environment script disables cgo for the Linux helper programs, and the packager enables it for the macOS child build.

For a smaller GUI capture/system-voice build without PocketTTS models:

```sh
source scripts/macos-cross-env.sh arm64
GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -tags gui -o dist/foreverdubbed ./cmd/foreverdubbed
```

Run that binary with `-tts system` or `-mute`. Omit `-tags gui` for a terminal-only build. The default PocketTTS backend requires the full native build above.

## Persistent self-signed identity

Run `bash scripts/setup-macos-signing.sh` once on Linux or macOS. It installs checksum-pinned `rcodesign` 0.29.0 and creates `.runtime/macos-signing/identity.pem`, containing a private key and a ten-year self-signed code-signing certificate. Re-running setup reuses the identity. The file is mode `0600`, and `.runtime/` is Git-ignored. Back it up privately outside this checkout: deleting or replacing it changes the app's identity and requires granting capture permission again.

The regular release builder now signs the entire `.app`, including bundled dylibs and its resource manifest, before making the ZIP. It uses the existing `io.foreverdubbed.companion` bundle ID and a designated requirement tied to the certificate fingerprint. Do not edit files inside the signed app after packaging; use an external `-voice-config` for customization instead.

To reuse the same identity on another build machine, transfer it privately and set `FDB_MACOS_SIGNING_PEM` to its absolute path. Run `bash scripts/setup-macos-signing.sh --tools-only` to install the signer without creating a new identity. `FDB_RCODESIGN` can select a different signer executable. The builder fails if the identity or signer is missing; it never silently generates a replacement. `-mac-unsigned` explicitly opts out for disposable test builds, which will not preserve the signed app's permission identity.

After switching from the old ad-hoc build:

1. Quit ForeverDubbed from its tray menu and replace the old app in `/Applications` with the signed app.
2. Remove the old ForeverDubbed entry from Screen Recording settings once, add the new app, and approve it.
3. Quit and reopen it. Future builds signed with this same identity should retain approval; do not alternate them with unsigned builds.

On macOS, verify a downloaded or copied bundle with:

```sh
codesign --verify --deep --strict --verbose=2 /Applications/ForeverDubbed.app
codesign -d -r- /Applications/ForeverDubbed.app
```

The designated requirement should contain `io.foreverdubbed.companion` and a certificate hash, not a build-specific `cdhash`. Cross-builds verify the main executable with `rcodesign`; Apple's signature validation and actual permission persistence still need checking on a Mac. Self-signing does not remove Gatekeeper's unidentified-developer warning or provide notarization. No Keychain trust settings or Screen Recording permissions are changed by the setup script.

## GitHub Actions

[The macOS workflow](../.github/workflows/macos.yml) builds both architectures on `ubuntu-24.04`, runs the portable Go tests/vet, caches the pinned OSXCross toolchain, and uploads the two release ZIPs. It runs for pull requests, pushes to `main`, and manual dispatch. Pushes and manual runs sign with the `FDB_MACOS_SIGNING_PEM` Actions repository secret. Set its value to the entire contents of the existing `.runtime/macos-signing/identity.pem`, including both PEM blocks. The workflow installs only the signer, writes the identity to a restricted temporary file for the build, and removes that file on step exit, including after a build failure. A missing secret fails the signed build instead of silently switching identities. Pull-request builds never receive the secret: they explicitly use `-mac-unsigned` and label their artifacts with `-unsigned`. Those test artifacts are not permission-preserving updates to the signed app. It does not publish releases or run macOS executables on Linux. Native screen/audio behavior must be checked on a Mac.

The workflow uses Go 1.27.1 and matching LLVM 18 packages. It adds LLVM's `bin` directory to `GITHUB_PATH` before toolchain setup or cache restore, so subsequent steps can find `ld64.lld` and the other LLVM tools. Download the `ForeverDubbed-darwin-arm64` (Apple Silicon) or `ForeverDubbed-darwin-amd64` (Intel) artifact from a push or manual workflow run for its signed release ZIP. Pull-request artifacts have an additional `-unsigned` suffix.

## Build directly on a Mac

Install Go 1.27.1+, CMake 3.28+, Git, and Xcode command-line tools with a macOS 14+ SDK. From the repository root:

```sh
xcode-select --install
bash scripts/setup-macos-signing.sh
go run ./tools/native -target darwin -out .runtime/native-darwin
go run ./tools/models -out .runtime/native-darwin
go run ./tools/build -target darwin -native-dir .runtime/native-darwin
```

The default architecture matches the Mac. The resulting release ZIP has the same layout as a Linux cross-build.

## Run and grant capture permission

Extract the ZIP, move **ForeverDubbed.app** to Applications if desired, and double-click it. No terminal or external runtime folder is required. The addon remains beside the app in the ZIP; install it into the game separately.

On the first capture attempt, allow **Screen Recording** (called **Screen & System Audio Recording** on some versions) for **ForeverDubbed** under **System Settings → Privacy & Security**. Quit using the tray menu and reopen the app after granting access. Keep WoW visible and use `/fdb unlock` to display the tile.

The GUI displays capture, tile, and audio status with a **Stop** button beneath Audio that is enabled only during playback. Closing or minimizing it keeps it running in the menu bar; use **Show ForeverDubbed** to restore it or **Quit** to exit.

Enable **Queue new dialogue** to play incoming messages in order. Leave it unchecked
(the default) to interrupt speech and play the newest message. The app remembers
this setting. Unchecking it with messages waiting interrupts the current speech
and plays only the newest queued message. **Stop** skips the current speech;
in queue mode, the next waiting message then plays. Queue mode also shows a
**Skip** button: it advances to the next message, or stops if nothing else is
queued. Skip is enabled only during playback.

Local release builds are signed with a persistent self-signed certificate. They are not Developer ID signed or notarized. If macOS blocks a downloaded build, review its source and use [**System Settings → Privacy & Security → Open Anyway**](https://support.apple.com/en-gb/102445) for that app. Public notarized distribution requires a separate signing/notarization step.

For optional terminal diagnostics, open Terminal and select the embedded executable (adjust the path if the app is elsewhere):

```sh
fdb="/Applications/ForeverDubbed.app/Contents/MacOS/foreverdubbed"
"$fdb" -headless -mute
```

Useful checks:

```sh
"$fdb" -tts system -voices
"$fdb" -headless -tts system
"$fdb" -speak-test 'Testing PocketTTS playback.'
"$fdb" -snapshot capture.png
"$fdb" -image capture.png
```

Capture is restricted to a window owned by the **World of Warcraft Beta.app** bundle. The default matches the `.app` bundle containing the owning process's executable, so the retail client or a browser window titled World of Warcraft cannot be selected. The expected game executable is `/Applications/World of Warcraft/_classic_beta_/World of Warcraft Beta.app/Contents/MacOS/World of Warcraft`. It waits if the game is closed, minimized, or unavailable, and automatically rediscovers the window after it reopens or changes size. It never falls back to desktop capture. If more than one matching game process is open, close the other instance.

If your game's app bundle differs, specify its exact bundle name, absolute app/executable path, application name, or bundle identifier:

```sh
"$fdb" -capture-app "World of Warcraft Beta.app"
# To target this particular installation:
"$fdb" -capture-app "/Applications/World of Warcraft/_classic_beta_/World of Warcraft Beta.app"
```

ScreenCaptureKit captures the selected game window at native pixel scale. The reader crops the detected tile from that image in memory; neither the desktop nor other applications' windows are included. Coordinates are relative to the game window. `-snapshot` also saves only the selected game window. No images are saved unless you explicitly request a snapshot.

Before distributing a build, check on a Mac that the tile decodes in windowed and full-screen modes, that covering the game with another app does not add that app to `-snapshot`, and that closing/reopening or resizing the game recovers. Also check that long speech finishes without losing its ending, and new dialogue/Ctrl+C interrupts playback. Linux compilation and tests cannot verify Screen Recording permission, Retina capture, or speaker output.

For an opt-in native startup regression test on a Mac with the game open and Screen Recording permission granted, run:

```sh
FDB_TEST_CAPTURE_APP="World of Warcraft Beta.app" CGO_ENABLED=1 go test ./internal/platform -run TestMacWindowCaptureStartup -count=1
```

This exercises the real command-line CoreGraphics initialization and single-window capture path. The captured game image stays in memory.

## GUI validation

The release uses Go 1.27.1 and Fyne 2.8.1. On Linux, `go test -tags "gui ci" ./internal/desktop ./cmd/foreverdubbed` tests the dashboard using Fyne's software driver without an X server. The regular `go test ./...` suite remains independent of GUI development libraries.

On each Mac architecture, check opening the GUI, closing it to the menu bar, the native minimize button, Show ForeverDubbed, and Quit while speech is playing. Also check startup with missing models or denied capture permission: errors should appear in the dashboard. These native window/tray behaviors require live Mac testing.

## App bundle layout

The release builder creates the `.app` on Linux as well as macOS:

```text
ForeverDubbed.app/
  Contents/
    Info.plist
    MacOS/foreverdubbed
    Resources/
      app.icns
      native/              # ONNX dylibs, models, presets, licenses
      tts/                 # voices.json and configured custom voices
```

The executable finds models and voice configuration relative to its own location, so Finder launches and moving the app do not depend on the working directory. Its loader paths include `Contents/Resources/native`. To customize voices, copy the `tts` directory outside the app and use `-voice-config` to select that external configuration when launching from Terminal. Editing resources inside the app invalidates its signature.
