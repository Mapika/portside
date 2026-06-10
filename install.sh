#!/bin/sh
# portside installer: curl -fsSL https://raw.githubusercontent.com/Mapika/portside/main/install.sh | sh
set -eu

REPO="Mapika/portside"
BIN_DIR="${PORTSIDE_BIN_DIR:-$HOME/.local/bin}"

case "$(uname -s)" in
    Linux)  OS=linux  ;;
    Darwin) OS=darwin ;;
    *)
        echo "unsupported OS: $(uname -s)" >&2
        exit 1
        ;;
esac

ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH=amd64 ;;
    aarch64 | arm64) ARCH=arm64 ;;
    *)
        echo "unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac

URL="https://github.com/$REPO/releases/latest/download/portside_${OS}_${ARCH}.tar.gz"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "downloading $URL"
curl -fsSL "$URL" -o "$TMP/portside.tar.gz"
tar -xzf "$TMP/portside.tar.gz" -C "$TMP"

mkdir -p "$BIN_DIR"
install -m 755 "$TMP/portside" "$BIN_DIR/portside"
install -m 755 "$TMP/scripts/work" "$BIN_DIR/work"

echo "installed portside and work to $BIN_DIR"
case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *) echo "note: add $BIN_DIR to your PATH" ;;
esac
