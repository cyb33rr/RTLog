#!/usr/bin/env bash
# install.sh - Bootstrap installer for Red Team Operation Logger
# Downloads the pre-built binary and runs `rtlog setup`.
# Usage: curl -fsSL https://raw.githubusercontent.com/cyb33rr/RTLog/main/install.sh | bash
set -euo pipefail

REPO_RELEASE="https://github.com/cyb33rr/RTLog/releases/latest/download"
RT_DIR="$HOME/.rt"

echo "=== Red Team Operation Logger - Bootstrap Installer ==="
echo

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64)        ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "[!] Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

BINARY_NAME="rtlog-${OS}-${ARCH}"
BINARY_DST="$RT_DIR/rtlog"

mkdir -p "$RT_DIR"

echo "[*] Downloading rtlog ($OS/$ARCH)..."
curl -fsSL "$REPO_RELEASE/$BINARY_NAME" -o "$BINARY_DST" || {
    echo "[!] Failed to download binary" >&2; exit 1
}
chmod +x "$BINARY_DST"
echo "[+] Downloaded binary: $BINARY_DST"
echo

# Run self-contained setup
exec "$BINARY_DST" setup
