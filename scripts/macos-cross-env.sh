# Source this file: source scripts/macos-cross-env.sh arm64 [OSXCROSS_TARGET]
# amd64 builds for Intel; arm64 builds for Apple Silicon.
fdb_macos_cross_env() {
    local arch=${1:-arm64}
    local target=${2:-"$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.runtime/osxcross/target"}
    local machine
    case "$arch" in
        arm64) machine=arm64 ;;
        amd64) machine=x86_64 ;;
        *) echo 'Architecture must be arm64 or amd64' >&2; return 1 ;;
    esac
    local compilers=("$target/bin/$machine"-apple-darwin*[0-9]-clang)
    local sdks=("$target/SDK/MacOSX"*.sdk)
    if [[ ${#compilers[@]} != 1 || ! -x ${compilers[0]} || ${#sdks[@]} != 1 || ! -d ${sdks[0]} ]]; then
        echo "Expected one OSXCross compiler and SDK in $target; run scripts/setup-macos-cross.sh first." >&2
        return 1
    fi
    # OSXCross's LLVM wrappers also need these tools after a cache restore,
    # when setup-macos-cross.sh has not run in the current shell.
    local llvm_bin
    llvm_bin=$(llvm-config --bindir) || return 1
    export PATH="$target/bin:$llvm_bin:$PATH"
    export CC="${compilers[0]}" CXX="${compilers[0]}++" MACOS_SDK="${sdks[0]}"
    export MACOSX_DEPLOYMENT_TARGET=14.0
    # Host-side Go tools (notably net/http in tools/models) must not use
    # the macOS compiler. tools/build enables cgo for its target child.
    export CGO_ENABLED=0
    # Leave GOOS/GOARCH unset: go run executes the build tools on Linux.
}
fdb_macos_cross_env "$@"
