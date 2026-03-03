#!/usr/bin/env bash
# uninstall.sh - Remove Red Team Operation Logger artifacts
set -euo pipefail

LOCAL_BIN="$HOME/.local/bin"
ZSHRC="$HOME/.zshrc"
RT_DIR="$HOME/.rt"

SKIP_PROMPT=false
if [[ "${1:-}" == "-y" || "${1:-}" == "--yes" ]]; then
    SKIP_PROMPT=true
fi

confirm() {
    if $SKIP_PROMPT; then return 0; fi
    if [[ ! -t 0 ]]; then
        echo "(non-interactive, skipping: $1)"
        return 1
    fi
    printf "%s [y/N] " "$1"
    read -r answer < /dev/tty
    [[ "$answer" =~ ^[Yy]$ ]]
}

echo "=== Red Team Operation Logger - Uninstaller ==="
echo

# 1. Remove symlink
if [[ -L "$LOCAL_BIN/rtlog" ]]; then
    target="$(readlink "$LOCAL_BIN/rtlog")"
    if [[ "$target" == "$RT_DIR/rtlog" ]]; then
        rm "$LOCAL_BIN/rtlog"
        echo "[-] Removed symlink: $LOCAL_BIN/rtlog"
    else
        echo "[!] $LOCAL_BIN/rtlog points to $target (not ours), skipping"
    fi
elif [[ -e "$LOCAL_BIN/rtlog" ]]; then
    echo "[!] $LOCAL_BIN/rtlog exists but is not a symlink, skipping"
else
    echo "[ok] No symlink at $LOCAL_BIN/rtlog"
fi

# 2. Remove source line + comment from .zshrc
if [[ -f "$ZSHRC" ]]; then
    # Match both old (repo-based) and new (~/.rt/) source lines
    if grep -qE "source .*(/rtlog/hook\.zsh|\.rt/hook\.zsh)" "$ZSHRC" 2>/dev/null; then
        sed -i "\|# Red Team Operation Logger|d" "$ZSHRC"
        sed -i "\|source .*hook\.zsh|d" "$ZSHRC"
        echo "[-] Removed hook lines from $ZSHRC"
    else
        echo "[ok] No hook lines in $ZSHRC"
    fi
else
    echo "[ok] No $ZSHRC found"
fi

# 3. Remove ~/.rt/ (prompt first - contains engagement logs and runtime files)
if [[ -d "$RT_DIR" ]]; then
    if confirm "Delete $RT_DIR? This contains runtime files and all engagement logs."; then
        rm -rf "$RT_DIR"
        echo "[-] Removed $RT_DIR"
    else
        echo "[ok] Kept $RT_DIR"
    fi
else
    echo "[ok] No $RT_DIR directory"
fi

echo
echo "=== Uninstall complete ==="
echo
echo "Run 'source ~/.zshrc' or open a new shell to apply changes."
