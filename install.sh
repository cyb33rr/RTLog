#!/usr/bin/env bash
# install.sh - Idempotent installer for Red Team Operation Logger (Go version)
set -euo pipefail

REPO_RELEASE="https://github.com/cyb33rr/RTLog/releases/latest/download"
RUNTIME_FILES=(hook.zsh tools.conf uninstall.sh)

RT_DIR="$HOME/.rt"
LOG_DIR="$RT_DIR/logs"
LOCAL_BIN="$HOME/.local/bin"
ZSHRC="$HOME/.zshrc"
SOURCE_LINE="source $RT_DIR/hook.zsh"

echo "=== Red Team Operation Logger - Installer ==="
echo

# Determine source directory
if [[ -e "./hook.zsh" && -e "./tools.conf" ]]; then
    SCRIPT_DIR="$(pwd)"
    echo "[*] Installing from local copy..."
else
    SCRIPT_DIR="$(mktemp -d)"
    trap 'rm -rf "$SCRIPT_DIR"' EXIT
    echo "[*] Downloading files from GitHub..."
    for f in "${RUNTIME_FILES[@]}"; do
        curl -fsSL "$REPO_RELEASE/../main/$f" -o "$SCRIPT_DIR/$f" || { echo "[!] Failed to download $f" >&2; exit 1; }
    done
fi

# 1. Create directories
if [[ -d "$LOG_DIR" ]]; then
    echo "[ok] Log directory exists: $LOG_DIR"
else
    mkdir -p "$LOG_DIR"
    echo "[+]  Created log directory: $LOG_DIR"
fi

# 2. Build or download rtlog binary
BINARY_DST="$RT_DIR/rtlog"
if [[ -e "./go.mod" ]] && command -v go &>/dev/null; then
    echo "[*] Building rtlog from source..."
    go build -ldflags "-s -w" -o "$BINARY_DST" .
    echo "[+]  Built binary: $BINARY_DST"
else
    # Detect platform
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"
    case "$ARCH" in
        x86_64)  ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) echo "[!] Unsupported architecture: $ARCH" >&2; exit 1 ;;
    esac
    BINARY_NAME="rtlog-${OS}-${ARCH}"
    echo "[*] Downloading pre-built binary ($OS/$ARCH)..."
    curl -fsSL "$REPO_RELEASE/$BINARY_NAME" -o "$BINARY_DST" || { echo "[!] Failed to download binary" >&2; exit 1; }
    chmod +x "$BINARY_DST"
    echo "[+]  Downloaded binary: $BINARY_DST"
fi

# 3. Copy runtime files into ~/.rt/
for f in "${RUNTIME_FILES[@]}"; do
    src="$SCRIPT_DIR/$f"
    dst="$RT_DIR/$f"
    if [[ ! -e "$src" ]]; then
        echo "[!]  Missing source file: $src" >&2; exit 1
    fi
    if [[ -e "$dst" ]] && cmp -s "$src" "$dst"; then
        echo "[ok] $f is up to date"
    else
        cp "$src" "$dst"
        echo "[+]  Installed $f -> $dst"
    fi
done
chmod +x "$RT_DIR/uninstall.sh"

# 4. Symlink rtlog to ~/.local/bin/
mkdir -p "$LOCAL_BIN"
if [[ -L "$LOCAL_BIN/rtlog" ]]; then
    current_target="$(readlink "$LOCAL_BIN/rtlog")"
    if [[ "$current_target" == "$RT_DIR/rtlog" ]]; then
        echo "[ok] Symlink already exists: $LOCAL_BIN/rtlog -> $RT_DIR/rtlog"
    else
        ln -sf "$RT_DIR/rtlog" "$LOCAL_BIN/rtlog"
        echo "[+]  Updated symlink: $LOCAL_BIN/rtlog -> $RT_DIR/rtlog"
    fi
elif [[ -e "$LOCAL_BIN/rtlog" ]]; then
    echo "[!]  $LOCAL_BIN/rtlog exists but is not a symlink, skipping"
else
    ln -s "$RT_DIR/rtlog" "$LOCAL_BIN/rtlog"
    echo "[+]  Created symlink: $LOCAL_BIN/rtlog -> $RT_DIR/rtlog"
fi

# 5. Ensure ~/.local/bin is in PATH
if echo "$PATH" | tr ':' '\n' | grep -qx "$LOCAL_BIN"; then
    echo "[ok] $LOCAL_BIN is already in PATH"
else
    if grep -qF 'export PATH="$HOME/.local/bin' "$ZSHRC" 2>/dev/null; then
        echo "[ok] PATH entry for $LOCAL_BIN already in $ZSHRC"
    else
        printf '\nexport PATH="$HOME/.local/bin:$PATH"\n' >> "$ZSHRC"
        echo "[+]  Added $LOCAL_BIN to PATH in $ZSHRC"
    fi
fi

# 6. Add source line to .zshrc (idempotent)
# Remove old source lines pointing to the git repo or prior install locations
if grep -qE "source .*(rtlog|python-hook)/hook\.zsh" "$ZSHRC" 2>/dev/null; then
    sed -i "\|source .*/rtlog/hook\.zsh|d" "$ZSHRC"
    sed -i "\|source .*/python-hook/hook\.zsh|d" "$ZSHRC"
    sed -i '/^# Red Team Operation Logger$/{ N; /\n$/d; }' "$ZSHRC"
    echo "[+]  Removed old hook source line(s) from $ZSHRC"
fi

if grep -qF "$SOURCE_LINE" "$ZSHRC" 2>/dev/null; then
    echo "[ok] hook.zsh already sourced in $ZSHRC"
else
    printf '\n# Red Team Operation Logger\n%s\n' "$SOURCE_LINE" >> "$ZSHRC"
    echo "[+]  Added source line to $ZSHRC"
fi

echo
echo "=== Installation complete ==="
echo
echo "All runtime files installed to $RT_DIR/"
echo
echo "Quick-start:"
echo "  1. Reload shell:     source ~/.zshrc"
echo "  2. Start engagement: rtlog new <name>"
echo "  3. Set phase tag:    rtlog tag recon"
echo "  4. Run tools normally - logging is automatic"
echo "  5. Query logs:       rtlog show"
echo "  6. Full status:      rtlog status"
echo "  7. More commands:    rtlog --help"
