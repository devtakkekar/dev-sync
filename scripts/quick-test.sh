#!/usr/bin/env bash
# Dev-Sync Quick Test & Demonstration Script for Linux
set -euo pipefail

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RESET='\033[0m'

echo -e "${BLUE}========================================${RESET}"
echo -e "${BLUE}        Dev-Sync Linux Test Sandbox     ${RESET}"
echo -e "${BLUE}========================================${RESET}"

# Find dev-sync binary (current directory, dist/, or in PATH)
BIN=""
if [ -f "./dev-sync" ]; then
    BIN="./dev-sync"
elif [ -f "./dist/dev-sync-linux-arm64" ] && [ "$(uname -m)" = "aarch64" ]; then
    BIN="./dist/dev-sync-linux-arm64"
elif [ -f "./dist/dev-sync-linux-amd64" ] && [ "$(uname -m)" = "x86_64" ]; then
    BIN="./dist/dev-sync-linux-amd64"
elif command -v dev-sync >/dev/null 2>&1; then
    BIN="dev-sync"
fi

if [ -z "$BIN" ]; then
    echo -e "${YELLOW}Could not locate dev-sync executable.${RESET}"
    echo "Please place the dev-sync binary in this directory or install it in your PATH."
    exit 1
fi

chmod +x "$BIN"
echo -e "${GREEN}✓ Using binary:${RESET} $BIN ($($BIN version))"

SANDBOX_DIR="$(mktemp -d -t devsync-sandbox-XXXXXX)"
SOURCE_DIR="$SANDBOX_DIR/source"
REP1_DIR="$SANDBOX_DIR/replica-1"
REP2_DIR="$SANDBOX_DIR/replica-2"

echo -e "\n${BLUE}[1/5] Setting up temporary sandbox...${RESET}"
mkdir -p "$SOURCE_DIR/config" "$REP1_DIR" "$REP2_DIR"
echo "console.log('Hello Dev-Sync on Linux!');" > "$SOURCE_DIR/main.js"
echo '{"env": "test", "version": 1}' > "$SOURCE_DIR/config/app.json"
echo -e "${GREEN}✓ Sandbox created at:${RESET} $SANDBOX_DIR"

echo -e "\n${BLUE}[2/5] Initializing Dev-Sync...${RESET}"
cd "$SOURCE_DIR"
"$BIN" init --non-interactive --yes
"$BIN" add --id "replica-1" --path "$REP1_DIR"
"$BIN" add --id "replica-2" --path "$REP2_DIR"
"$BIN" doctor

echo -e "\n${BLUE}[3/5] Performing Initial Synchronization...${RESET}"
"$BIN" sync

echo -e "\n${BLUE}[4/5] Testing Modification & Drift Auto-Repair...${RESET}"
# Simulate source update
echo "console.log('Updated in source!');" > "$SOURCE_DIR/main.js"
# Simulate replica drift
echo "I am a rogue file" > "$REP1_DIR/unwanted.txt"

echo "Checking status..."
"$BIN" status

echo "Synchronizing and healing drift..."
"$BIN" sync

echo -e "\n${BLUE}[5/5] Cryptographic Verification...${RESET}"
"$BIN" verify

echo -e "\n${GREEN}========================================${RESET}"
echo -e "${GREEN}  All tests completed successfully!     ${RESET}"
echo -e "${GREEN}========================================${RESET}"
echo -e "Sandbox location: $SANDBOX_DIR"
echo -e "To test live watch mode, run:"
echo -e "  cd $SOURCE_DIR && $BIN watch --auto-repair"
