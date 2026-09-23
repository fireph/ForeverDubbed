#!/usr/bin/env bash
# Install a pinned cross-platform signer and optionally create a reusable identity.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
mode=${1:-identity}
if [[ "$mode" != identity && "$mode" != --tools-only ]]; then
    echo "Usage: $0 [--tools-only]" >&2
    exit 1
fi
version=0.29.0
case "$(uname -s)/$(uname -m)" in
    Linux/aarch64|Linux/arm64)
        target=aarch64-unknown-linux-musl
        checksum=4af92c87ddf52f5f2d1258a3b4e56c7dcb8f1b2468df744976c5f139e031961f ;;
    Linux/x86_64)
        target=x86_64-unknown-linux-musl
        checksum=dbe85cedd8ee4217b64e9a0e4c2aef92ab8bcaaa41f20bde99781ff02e600002 ;;
    Darwin/arm64|Darwin/x86_64)
        target=macos-universal
        checksum=d98372d5524226ccf9dc0eda03d4e4f5826182dabb2fc3f2bd303ed9113a748d ;;
    *) echo 'Signing setup supports Linux and macOS on arm64/amd64.' >&2; exit 1 ;;
esac
tool_dir="$root/.runtime/rcodesign/$version"
signer="$tool_dir/rcodesign"
if [[ ! -x "$signer" ]]; then
    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT
    archive="apple-codesign-$version-$target.tar.gz"
    curl -fL "https://github.com/indygreg/apple-platform-rs/releases/download/apple-codesign/$version/$archive" -o "$tmp/archive.tar.gz"
    actual=$(shasum -a 256 "$tmp/archive.tar.gz" | awk '{print $1}')
    [[ "$actual" == "$checksum" ]] || { echo 'rcodesign checksum mismatch' >&2; exit 1; }
    tar -xzf "$tmp/archive.tar.gz" -C "$tmp"
    mkdir -p "$tool_dir"
    cp "$tmp/apple-codesign-$version-$target/rcodesign" "$signer"
    chmod 755 "$signer"
fi
"$signer" --version
if [[ "$mode" == --tools-only ]]; then exit 0; fi
if [[ -n "${CI:-}" ]]; then
    echo 'CI must restore the existing signing identity, not generate a new one. See docs/macos.md.' >&2
    exit 1
fi
identity="$root/.runtime/macos-signing/identity.pem"
umask 077
mkdir -p "$(dirname "$identity")"
if [[ -e "$identity" ]]; then
    echo "Reusing existing identity: $identity"
else
    # noclobber prevents concurrent setup invocations replacing an identity.
    (set -o noclobber; : > "$identity")
    tmp_identity=$(mktemp "$(dirname "$identity")/.identity.XXXXXX")
    trap 'rm -f "$tmp_identity"' EXIT
    "$signer" --config-file /dev/null generate-self-signed-certificate \
        --person-name 'ForeverDubbed Local Signing' --validity-days 3650 \
        --pem-unified-file "$tmp_identity"
    mv "$tmp_identity" "$identity"
    echo "Created identity: $identity"
fi
# Validate without ever printing the private key.
openssl x509 -in "$identity" -noout -subject -fingerprint -sha256
openssl pkey -in "$identity" -check -noout >/dev/null
chmod 600 "$identity"
echo 'Back up identity.pem privately. Reuse it for all builds; never commit it.'
