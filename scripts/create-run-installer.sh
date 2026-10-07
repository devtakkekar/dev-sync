#!/usr/bin/env bash
# Creates a standalone .run self-extracting installer for a target binary
set -euo pipefail

BINARY_PATH="${1:-}"
OUTPUT_PATH="${2:-}"
ARCH="${3:-linux}"

if [ -z "$BINARY_PATH" ] || [ ! -f "$BINARY_PATH" ]; then
    echo "Usage: $0 <binary-path> <output-run-path> [arch]"
    exit 1
fi

TMP_HEADER="$(mktemp)"

cat << 'EOF' > "$TMP_HEADER"
#!/bin/sh
# ==============================================================================
# Dev-Sync Self-Extracting Linux Installer (.run)
# ==============================================================================
set -e

VERSION="1.0.0"
APP_NAME="dev-sync"

show_help() {
    echo "Dev-Sync v$VERSION Installer"
    echo "Usage: $0 [options]"
    echo ""
    echo "Options:"
    echo "  --prefix <dir>   Custom installation directory (default: /usr/local/bin)"
    echo "  --extract-only   Extract binary into current folder without system install"
    echo "  -h, --help       Display this help message"
    exit 0
}

TARGET_DIR="/usr/local/bin"
EXTRACT_ONLY=0

while [ $# -gt 0 ]; do
    case "$1" in
        --prefix)
            TARGET_DIR="$2"
            shift 2
            ;;
        --extract-only)
            EXTRACT_ONLY=1
            shift
            ;;
        -h|--help)
            show_help
            ;;
        *)
            echo "Unknown option: $1"
            show_help
            ;;
    esac
done

echo "=========================================="
echo "    Dev-Sync Linux Installer v$VERSION    "
echo "=========================================="

PAYLOAD_LINE=$(awk '/^__PAYLOAD_BEGINS_HERE__/ {print NR + 1; exit 0; }' "$0")
TMP_BIN=$(mktemp)

if ! tail -n +"$PAYLOAD_LINE" "$0" | tr -d '\r' | base64 -d -i > "$TMP_BIN" 2>/dev/null; then
    tail -n +"$PAYLOAD_LINE" "$0" | tr -d '\r' | base64 -d > "$TMP_BIN"
fi

chmod +x "$TMP_BIN"

if [ "$EXTRACT_ONLY" -eq 1 ]; then
    cp "$TMP_BIN" "./$APP_NAME"
    chmod +x "./$APP_NAME"
    rm -f "$TMP_BIN"
    echo "✓ Extracted binary to ./$APP_NAME"
    exit 0
fi

SUDO=""
if [ ! -w "$TARGET_DIR" ] && [ "$(id -u)" -ne 0 ]; then
    if command -v sudo >/dev/null 2>&1; then
        SUDO="sudo"
    else
        TARGET_DIR="$HOME/.local/bin"
        mkdir -p "$TARGET_DIR"
    fi
fi

$SUDO install -m 755 "$TMP_BIN" "$TARGET_DIR/$APP_NAME"
rm -f "$TMP_BIN"

echo "✓ Successfully installed $APP_NAME to $TARGET_DIR/$APP_NAME"
"$TARGET_DIR/$APP_NAME" version 2>/dev/null || true
echo "Run '$APP_NAME --help' to get started."
exit 0

__PAYLOAD_BEGINS_HERE__
EOF

# Combine header and base64 encoded payload
cat "$TMP_HEADER" > "$OUTPUT_PATH"
base64 "$BINARY_PATH" >> "$OUTPUT_PATH"
chmod +x "$OUTPUT_PATH"
rm -f "$TMP_HEADER"

echo "✓ Created $OUTPUT_PATH successfully."
