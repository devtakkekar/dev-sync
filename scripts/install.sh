#!/usr/bin/env bash
# Dev-Sync One-Line Installer for Linux
set -euo pipefail

ARCH="$(uname -m)"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"

if [ "$OS" != "linux" ]; then
    echo "Error: This installer is intended for Linux systems."
    exit 1
fi

case "$ARCH" in
    x86_64)
        TARGET_ARCH="amd64"
        ;;
    aarch64|arm64)
        TARGET_ARCH="arm64"
        ;;
    *)
        echo "Error: Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

BINARY_NAME="dev-sync-linux-${TARGET_ARCH}"
INSTALL_DIR="/usr/local/bin"

if [ ! -w "$INSTALL_DIR" ]; then
    SUDO="sudo"
else
    SUDO=""
fi

echo "Detected Linux architecture: $TARGET_ARCH"

# Check if local binary exists in dist/ or current directory
if [ -f "./dist/$BINARY_NAME" ]; then
    echo "Installing from local dist/$BINARY_NAME..."
    $SUDO install -m 755 "./dist/$BINARY_NAME" "$INSTALL_DIR/dev-sync"
elif [ -f "./$BINARY_NAME" ]; then
    echo "Installing from local $BINARY_NAME..."
    $SUDO install -m 755 "./$BINARY_NAME" "$INSTALL_DIR/dev-sync"
else
    REPO="devsync/dev-sync"
    URL="https://github.com/$REPO/releases/latest/download/$BINARY_NAME"
    echo "Downloading dev-sync for Linux ($TARGET_ARCH)..."
    TMP_FILE="$(mktemp)"
    curl -fsSL "$URL" -o "$TMP_FILE"
    $SUDO install -m 755 "$TMP_FILE" "$INSTALL_DIR/dev-sync"
    rm -f "$TMP_FILE"
fi

echo "✓ dev-sync successfully installed to $INSTALL_DIR/dev-sync"
dev-sync version
