# hook.bash - Red Team Operation Logger (bash shell hook)
# Source this file in .bashrc to automatically log red team tool usage.
# Requires bash 4.2+ and bash-preexec.

[[ $- == *i* ]] || return 0
[[ -v _RTLOG_LOADED ]] && return 0
_RTLOG_LOADED=1

# --- Load bash-preexec ---
_rtlog_hook_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
# shellcheck source=bash-preexec.sh
source "${_rtlog_hook_dir}/bash-preexec.sh"

# --- Configuration ---
RTLOG_DIR="${RTLOG_DIR:-$HOME/.rt/logs}"
RTLOG_CONF="${_rtlog_hook_dir}/tools.conf"
RTLOG_ENABLED=1
RTLOG_ENGAGEMENT=""
RTLOG_TAG=""
RTLOG_NOTE=""
RTLOG_CAPTURE=${RTLOG_CAPTURE:-1}   # 1 = capture command output, 0 = metadata only

# --- State file ---
RTLOG_STATE_FILE="${HOME}/.rt/state"

_rtlog_sync_state() {
    [[ -r "$RTLOG_STATE_FILE" ]] || return 0
    local key val
    while IFS='=' read -r key val; do
        case "$key" in
            engagement) RTLOG_ENGAGEMENT="$val" ;;
            tag)        RTLOG_TAG="$val" ;;
            note)       RTLOG_NOTE="$val" ;;
            enabled)    RTLOG_ENABLED="$val" ;;
            capture)    RTLOG_CAPTURE="$val" ;;
        esac
    done < "$RTLOG_STATE_FILE"
}

_rtlog_write_state() {
    local dir
    dir="$(dirname "$RTLOG_STATE_FILE")"
    [[ -d "$dir" ]] || mkdir -p "$dir"
    local tmp="${RTLOG_STATE_FILE}.tmp"
    printf 'engagement=%s\ntag=%s\nnote=%s\nenabled=%s\ncapture=%s\n' \
        "$RTLOG_ENGAGEMENT" "$RTLOG_TAG" "$RTLOG_NOTE" "$RTLOG_ENABLED" "$RTLOG_CAPTURE" > "$tmp"
    command mv -f "$tmp" "$RTLOG_STATE_FILE"
}

# Initial state sync from file (if exists)
_rtlog_sync_state

# --- Tool lookup tables ---
declare -gA _rtlog_tools_exact
declare -ga _rtlog_tools_glob

# --- Cache tty at source time ---
_rtlog_tty=$(tty 2>/dev/null) || true
[[ "$_rtlog_tty" == /dev/* ]] || _rtlog_tty="unknown"

# --- Temp file for output capture (per-shell PID) ---
_rtlog_tmpfile="/tmp/.rtlog_out.$$"

# --- Load tools.conf ---
_rtlog_load_conf() {
    _rtlog_tools_exact=()
    _rtlog_tools_glob=()

    [[ -r "$RTLOG_CONF" ]] || return 1

    local line
    while IFS= read -r line; do
        # Trim leading/trailing whitespace and tabs
        line="${line#"${line%%[![:space:]]*}"}"
        line="${line%"${line##*[![:space:]]}"}"
        [[ -z "$line" || "$line" == \#* ]] && continue
        if [[ "$line" == *'*'* || "$line" == *'?'* ]]; then
            _rtlog_tools_glob+=("$line")
        else
            _rtlog_tools_exact[$line]=1
        fi
    done < "$RTLOG_CONF"
}

_rtlog_load_conf

# --- Tool matching ---
_rtlog_match_tool() {
    [[ -v "_rtlog_tools_exact[$1]" ]] && return 0
    local pat
    for pat in "${_rtlog_tools_glob[@]}"; do
        # bash [[ ]] does glob matching natively
        [[ "$1" == $pat ]] && return 0
    done
    return 1
}

# --- Pending state ---
_rtlog_pending_tool=""
_rtlog_pending_cmd=""
_rtlog_pending_start=""
_rtlog_pending_rc=""
_rtlog_capturing=0
_rtlog_fd_out=""
_rtlog_fd_err=""

# --- preexec hook ---
_rtlog_preexec() {
    _rtlog_sync_state
    (( RTLOG_DEBUG )) && echo "[rtlog:preexec] cmd='$1' enabled=$RTLOG_ENABLED eng=$RTLOG_ENGAGEMENT" >&2
    [[ "$RTLOG_ENABLED" == "1" ]] || return
    [[ -n "$RTLOG_ENGAGEMENT" ]] || return

    local -a words
    read -ra words <<< "$1"

    while (( ${#words[@]} > 0 )); do
        case "${words[0]}" in
            *=*) words=("${words[@]:1}") ;;   # skip inline env var assignments
            sudo|nohup|time|env|command|exec|nice|ionice|strace|ltrace|proxychains|proxychains4|tsocks)
                words=("${words[@]:1}") ;;
            *) break ;;
        esac
    done

    (( ${#words[@]} > 0 )) || return

    local tool="${words[0]##*/}"

    if ! _rtlog_match_tool "$tool"; then
        (( RTLOG_DEBUG )) && echo "[rtlog:preexec] no match for '$tool'" >&2
        return
    fi

    (( RTLOG_DEBUG )) && echo "[rtlog:preexec] matched '$tool'" >&2

    _rtlog_pending_tool="$tool"
    _rtlog_pending_cmd="$1"
    _rtlog_pending_start="$(date +%s.%N)"
    _rtlog_pending_rc=""
    _rtlog_capturing=0

    # --- Output capture ---
    if [[ "$RTLOG_CAPTURE" == "1" ]]; then
        : > "$_rtlog_tmpfile"
        exec {_rtlog_fd_out}>&1 {_rtlog_fd_err}>&2
        exec > >(tee -- "$_rtlog_tmpfile") 2>&1
        _rtlog_capturing=1
        (( RTLOG_DEBUG )) && echo "[rtlog:preexec] capturing output" >&2
    fi
}

# --- Save $? and restore fds (runs FIRST in precmd) ---
_rtlog_save_rc() {
    _rtlog_pending_rc=$?
    if (( _rtlog_capturing )); then
        exec 1>&${_rtlog_fd_out} 2>&${_rtlog_fd_err}
        exec {_rtlog_fd_out}>&- {_rtlog_fd_err}>&-
        _rtlog_fd_out=""
        _rtlog_fd_err=""
        _rtlog_capturing=0
    fi
}

# --- JSON-escape a string ---
_rtlog_json_escape() {
    local s="$1"
    # Backslash and quote first (order matters)
    s="${s//\\/\\\\}"
    s="${s//\"/\\\"}"
    # Standard JSON escapes
    s="${s//$'\n'/\\n}"
    s="${s//$'\t'/\\t}"
    s="${s//$'\r'/\\r}"
    # All remaining control characters -> \u00XX
    s="${s//$'\x00'/\\u0000}"
    s="${s//$'\x01'/\\u0001}"
    s="${s//$'\x02'/\\u0002}"
    s="${s//$'\x03'/\\u0003}"
    s="${s//$'\x04'/\\u0004}"
    s="${s//$'\x05'/\\u0005}"
    s="${s//$'\x06'/\\u0006}"
    s="${s//$'\x07'/\\u0007}"
    s="${s//$'\x08'/\\u0008}"
    s="${s//$'\x0b'/\\u000b}"
    s="${s//$'\x0c'/\\u000c}"
    s="${s//$'\x0e'/\\u000e}"
    s="${s//$'\x0f'/\\u000f}"
    s="${s//$'\x10'/\\u0010}"
    s="${s//$'\x11'/\\u0011}"
    s="${s//$'\x12'/\\u0012}"
    s="${s//$'\x13'/\\u0013}"
    s="${s//$'\x14'/\\u0014}"
    s="${s//$'\x15'/\\u0015}"
    s="${s//$'\x16'/\\u0016}"
    s="${s//$'\x17'/\\u0017}"
    s="${s//$'\x18'/\\u0018}"
    s="${s//$'\x19'/\\u0019}"
    s="${s//$'\x1a'/\\u001a}"
    s="${s//$'\x1b'/\\u001b}"
    s="${s//$'\x1c'/\\u001c}"
    s="${s//$'\x1d'/\\u001d}"
    s="${s//$'\x1e'/\\u001e}"
    s="${s//$'\x1f'/\\u001f}"
    REPLY="$s"
}

# --- precmd hook ---
_rtlog_precmd() {
    (( RTLOG_DEBUG )) && echo "[rtlog:precmd] pending='$_rtlog_pending_tool'" >&2
    local rc=${_rtlog_pending_rc:-$?}

    [[ -n "$_rtlog_pending_tool" ]] || return

    # Duration (use awk for float arithmetic)
    local dur
    dur=$(awk "BEGIN {printf \"%.1f\", $(date +%s.%N) - $_rtlog_pending_start}")

    # Timestamp
    local ts epoch
    ts="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
    epoch="$(date +%s)"

    # Escape cmd
    _rtlog_json_escape "$_rtlog_pending_cmd"
    local escaped_cmd="$REPLY"

    # Escape note
    _rtlog_json_escape "$RTLOG_NOTE"
    local escaped_note="$REPLY"

    # Read and escape captured output
    local escaped_out=""
    if [[ -s "$_rtlog_tmpfile" ]]; then
        local raw_out
        raw_out="$(<"$_rtlog_tmpfile")"
        _rtlog_json_escape "$raw_out"
        escaped_out="$REPLY"
    fi
    command rm -f "$_rtlog_tmpfile" 2>/dev/null

    # Escape metadata fields that may contain quotes, backslashes, etc.
    _rtlog_json_escape "$USER"
    local escaped_user="$REPLY"
    _rtlog_json_escape "${HOSTNAME:-$(hostname)}"
    local escaped_host="$REPLY"
    _rtlog_json_escape "$_rtlog_tty"
    local escaped_tty="$REPLY"
    _rtlog_json_escape "$PWD"
    local escaped_cwd="$REPLY"
    _rtlog_json_escape "$RTLOG_TAG"
    local escaped_tag="$REPLY"

    # Ensure log directory exists
    [[ -d "$RTLOG_DIR" ]] || mkdir -p "$RTLOG_DIR"

    # Write JSONL entry (single line, everything inline)
    printf '{"ts":"%s","epoch":%d,"user":"%s","host":"%s","tty":"%s","cwd":"%s","tool":"%s","cmd":"%s","exit":%d,"dur":%s,"tag":"%s","note":"%s","out":"%s"}\n' \
        "$ts" "$epoch" "$escaped_user" "$escaped_host" "$escaped_tty" "$escaped_cwd" \
        "$_rtlog_pending_tool" "$escaped_cmd" "$rc" "$dur" "$escaped_tag" "$escaped_note" "$escaped_out" \
        >> "$RTLOG_DIR/${RTLOG_ENGAGEMENT}.jsonl"

    # Reset (note is one-shot — clear and write back to state file)
    local _had_note="$RTLOG_NOTE"
    RTLOG_NOTE=""
    _rtlog_pending_tool=""
    _rtlog_pending_cmd=""
    _rtlog_pending_start=""
    [[ -n "$_had_note" ]] && _rtlog_write_state
}

# --- Hook registration ---
precmd_functions=(_rtlog_save_rc "${precmd_functions[@]}")
preexec_functions+=(_rtlog_preexec)
precmd_functions+=(_rtlog_precmd)
