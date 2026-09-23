#!/usr/bin/env bash
# Linux host: build a pinned OSXCross toolchain with the macOS 14.5 SDK.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
destination=${1:-"$root/.runtime/osxcross"}
mkdir -p "$destination"
destination=$(cd "$destination" && pwd)
revision=27d21e4977c9751d01199c7a226a6faf494c3dd9
sdk_sha256=6e146275d19f027faa2e8354da5e0267513abf013b8f16ad65a231653a2b1c5d
for tool in clang clang++ llvm-config ld64.lld cmake git curl make xz; do
    command -v "$tool" >/dev/null || { echo "Missing $tool; see docs/macos.md for Linux prerequisites." >&2; exit 1; }
done
if [[ ! -d "$destination/.git" ]]; then
    git -C "$destination" init -q
    git -C "$destination" remote add origin https://github.com/tpoechtrager/osxcross.git
fi
git -C "$destination" fetch --depth 1 origin "$revision"
git -C "$destination" checkout --detach FETCH_HEAD
sdk="$destination/tarballs/MacOSX14.5.sdk.tar.xz"
if [[ ! -f "$sdk" ]]; then
    curl --fail --location --retry 3 https://github.com/joseluisq/macosx-sdks/releases/download/14.5/MacOSX14.5.sdk.tar.xz -o "$sdk"
fi
printf '%s  %s\n' "$sdk_sha256" "$sdk" | sha256sum --check
(
    cd "$destination"
    UNATTENDED=1 BUILD_FLAVOR=llvm ENABLE_ARCHS='arm64 x86_64' OSX_VERSION_MIN=14.0 ./build.sh
)
echo "OSXCross ready at $destination/target. See docs/macos.md for build commands."
