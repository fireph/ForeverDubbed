# Native speech runtime

PocketTTS.cpp and our C wrapper are compiled by `go build` through cgo and linked into the application executable. No PocketTTS DLL, `.so`, or `.dylib` is loaded or distributed. ONNX Runtime remains a separate shared library. No Python interpreter, subprocess, HTTP server, or network access is used during synthesis.

`internal/pocket/pocket_tts.hpp` is VolgaGerm/PocketTTS.cpp at commit `e801e7d6c2692121a39e80ae525cb5265174a495`, under `native/vendor/LICENSE`. The source uses a header extension so cgo compiles it once through `internal/pocket/bridge.cpp` and tracks changes for rebuilds. Local changes:

- Unused upstream HTTP server, command-line program, and alternate C API are removed; the application uses only its own `fdb_*` streaming ABI.
- `PocketTTS::load_voice_state` imports the existing April-model `.safetensors` voice states. No re-cloning or lossy conversion is performed.
- Correct restoration of dynamic snapshot shapes and initialization of decoder `first` flags for the pinned April export.
- Exceptions from generation/decoding join the worker before propagating to Go.
- Decoder batch limits also apply after generation finishes, so playback backpressure cannot turn the remaining audio into one oversized decode batch.
- Optional per-profile sentence fades use a continuous cosine envelope across decoder batches. Only the final fade-out window is retained before delivery; sample count is unchanged. Cancellation discards the retained tail. Both fades default to zero, which forwards audio unchanged.
- Ignore EOS predictions during the first six generated frames (480 ms), preventing leading pauses from ending short utterances before speech starts ([Kyutai #319](https://github.com/kyutai-labs/pocket-tts/pull/319)). The configured post-EOS tail is unchanged.
- Normalize sentence endings around quotes, brackets, percentages, and trailing commas/dashes before synthesis ([Kyutai #296](https://github.com/kyutai-labs/pocket-tts/pull/296)). Existing terminal punctuation and short-text padding are preserved.
- Windows UTF-8 path conversion allocates space for its terminator.

`bridge.cpp` owns the model and a bounded four-buffer PCM queue. The native decoder processes 15 latent frames (1.2 seconds of audio) per batch, including the first batch, and flushes shorter batches at the end of a sentence. First playback waits for that batch to be generated; the wall-clock delay depends on inference speed. The callback splits decoded audio into at most 100 ms PCM buffers so playback queues remain bounded. Go polls without blocking on model computation, and cancellation aborts generation and joins both native workers before reuse. Per-profile decode steps are retained. The application's `-cpu-threads` flag configures the native sessions once (default 1); old Python per-profile thread overrides are no longer used.

## Build

Install Go 1.27.1+, CMake 3.28+, Git, and C/C++17 compilers. Windows builds require an x64 MinGW-w64 GCC/G++ toolchain usable by cgo, with `gcc`, `g++`, and `mingw32-make` on PATH. MSVC alone is not a cgo toolchain. Linux uses GCC/G++ or Clang; macOS uses the Xcode command-line tools. Set `CC` and `CXX` if using different compiler names, consistently for dependency preparation and Go compilation.

```
go run ./tools/native
go run ./tools/models
go run ./tools/build
```

These commands target the Windows x64 release on every host. On Ubuntu 24.04+/WSL, install Go 1.27.1+ separately (the distro Go package may be older), then install the remaining prerequisites with:

```sh
sudo apt-get update
sudo apt-get install cmake git build-essential g++-mingw-w64-x86-64-posix
```

Both setup and packaging automatically select `x86_64-w64-mingw32-gcc-posix` / `x86_64-w64-mingw32-g++-posix` (or the unsuffixed MinGW-w64 commands if those are available). Set both `CC` and `CXX` to override this. Do not set `GOOS=windows` for `go run`: the setup/build tools themselves must run on Ubuntu; they select the Windows target for their child builds.

CMake caches are separated by host and target under `.runtime/native/build-<host-os>-<host-arch>-<target-os>-<target-arch>`, so existing Linux or MSVC caches in the old `build/` directory do not interfere. If changing compilers for the same host/target, remove that target's generated build directory first.

`tools/native` uses CMake to fetch pinned dependencies and build static SentencePiece. Headers and link libraries are installed into `.runtime/sdk/<os>_<arch>/`; runtime libraries and license notices go into `.runtime/native/`. Go compiles our bridge and PocketTTS.cpp itself. The `pocket_native` build tag enables this integration; the release builder sets `pocket_native,gui` and enables cgo. The desktop interface uses Fyne 2.8.1. Windows releases use the GUI subsystem so double-clicking does not open a console; terminal flags such as `-headless` attach to the parent console when available, and redirected output is preserved. Builds without this tag support model-free tests and setup tools, but return an explicit error if asked to synthesize speech.

The dependencies are ONNX Runtime 1.23.2, SentencePiece 0.2.1, nlohmann/json 3.12.0, and dr_libs revision `dfe8377631000664666519fdb83da193fd8037f4`. Windows may need Microsoft's Visual C++ x64 redistributable for ONNX Runtime. The Windows link uses static C++/GCC and thread runtimes, including with Ubuntu's POSIX MinGW-w64 toolchain. ONNX Runtime still uses its DLL through an import library. If a custom toolchain requires additional runtime DLLs, place them in the native runtime directory; the packager copies them beside the executable.

The packager defaults to Windows x64; `-target darwin -arch arm64|amd64` selects macOS. See [Linux cross-build and macOS setup](../docs/macos.md). The default setup prepares `.runtime/sdk/windows_amd64` and Windows DLLs in `.runtime/native`, including when run from Ubuntu/WSL. To use another runtime directory, pass `-out` to both `tools/native` and `tools/models`, then use the same path as `tools/build -native-dir`. Game-window capture/playback support Windows 10 version 1903+ and macOS 14+. Windows capture uses cgo with Windows Graphics Capture and D3D11; no extra capture libraries need to be downloaded.

Windows release layout:

```
foreverdubbed.exe                 # includes PocketTTS.cpp and SentencePiece
onnxruntime.dll
onnxruntime_providers_shared.dll
native/models/                   # pinned ONNX graphs and tokenizer
native/presets/                   # preset safetensors
native/licenses/
tts/voices.json
tts/custom/                      # custom safetensors
```

The packager also copies ONNX Runtime DLLs beside `dist/foreverdubbed.exe` for development launches. `-native-dir` selects model/preset data; shared-library discovery happens through the OS loader before Go starts.

For direct speech development on Linux/macOS, prepare host libraries with `go run ./tools/native -target host`; the default Windows dependencies cannot be linked into a Linux/macOS executable. Use `CGO_ENABLED=1` and `-tags pocket_native`. Add the absolute `.runtime/native` directory to `PATH` on Windows, `LD_LIBRARY_PATH` on Linux, or `DYLD_LIBRARY_PATH` on macOS. For example, on Linux:

```sh
go run ./tools/native -target host
go run ./tools/models
export CGO_ENABLED=1
export LD_LIBRARY_PATH="$PWD/.runtime/native${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
go run -tags pocket_native ./tools/voicecheck
FDB_TEST_NATIVE_DIR="$PWD/.runtime/native" go test -tags pocket_native -race ./internal/pocket
```

macOS app bundles keep their libraries and model files in `Contents/Resources/native` and voice configuration in `Contents/Resources/tts`. The executable locates these relative to itself for Finder launches. Standalone Linux/macOS executables also search `native/` beside themselves and `../.runtime/native`. Go's temporary executables used by `go run`/`go test` need the library-search environment above.

## Models and issue #12

`internal/pocket/assets.json` pins the April English ONNX bundle and public preset states by source revision and SHA-256. `go run ./tools/models` downloads and verifies them. The application also verifies the model hashes when opening the engine. Exporting models is not needed to run or build the default application.

[Upstream issue #12](https://github.com/VolgaGerm/PocketTTS.cpp/issues/12) describes the exporter selecting an obsolete config/checkpoint and the extra BOS-before-voice conditioning required by April models. We use KevinAHM's pinned `english_2026-04` export, not the old `b6369a24` weights. Saved voice states already contain BOS conditioning, so the adapter must not prepend it again. Direct WAV/MP3 conditioning is deliberately handled by the optional export tool, not the running app.

To regenerate models in a development environment, use [KevinAHM's April-aware exporter](https://github.com/KevinAHM/pocket-tts-onnx-export), select `english_2026-04`, export and quantize, then validate the graph layouts and regenerate the checked-in asset manifest. Replacing models from a different checkpoint without re-exporting the matching voice states is unsupported.

Models: [KevinAHM/pocket-tts-onnx](https://huggingface.co/KevinAHM/pocket-tts-onnx), CC-BY-4.0; original architecture and weights by Kyutai. Preset state files: [Kyutai](https://huggingface.co/kyutai/pocket-tts-without-voice-cloning), revision recorded in the manifest. Voice source/license details: [kyutai/tts-voices](https://huggingface.co/kyutai/tts-voices). Library licenses are shipped under `native/licenses/`.

## Validate real voices

```
go run -tags pocket_native ./tools/voicecheck
go run -tags pocket_native ./tools/voicecheck -voice undead_male
```

This writes WAV review clips and `report.json` to `.runtime/voice-samples/`. It checks non-silent streamed output, first audio before completion, cancellation, and reuse after cancellation. Clips are collected from PCM callbacks; the application itself uses only streaming playback. Timing excludes initial model loading; the sample tool uses two inference threads. For native error-recovery tests, set `FDB_TEST_NATIVE_DIR` to the absolute native runtime directory and run `go test -tags pocket_native ./internal/pocket`. On Linux/macOS the Go command needs cgo enabled and C/C++ compilers and the library-search environment above.
