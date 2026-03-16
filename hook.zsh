# hook.zsh - Red Team Operation Logger (zsh shell hook)
# Source this file in .zshrc to automatically log red team tool usage.

[[ -o interactive ]] || return 0
(( ${+_RTLOG_LOADED} )) && return 0
_RTLOG_LOADED=1

# --- Load modules ---
zmodload zsh/datetime
autoload -Uz add-zsh-hook

# --- Configuration ---
RTLOG_DIR="${RTLOG_DIR:-$HOME/.rt/logs}"
RTLOG_CONF="${0:A:h}/tools.conf"
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
    local dir="${RTLOG_STATE_FILE:h}"
    [[ -d "$dir" ]] || mkdir -p "$dir"
    local tmp="${RTLOG_STATE_FILE}.tmp"
    printf 'engagement=%s\ntag=%s\nnote=%s\nenabled=%s\ncapture=%s\n' \
        "$RTLOG_ENGAGEMENT" "$RTLOG_TAG" "$RTLOG_NOTE" "$RTLOG_ENABLED" "$RTLOG_CAPTURE" > "$tmp"
    command mv -f "$tmp" "$RTLOG_STATE_FILE"
}

# Initial state sync from file (if exists)
_rtlog_sync_state

# --- Tool lookup tables ---
typeset -gA _rtlog_tools_exact
typeset -ga _rtlog_tools_glob

# --- Cache tty at source time ---
_rtlog_tty=$(tty 2>/dev/null) || true
[[ "$_rtlog_tty" == /dev/* ]] || _rtlog_tty="unknown"

# --- Temp file for output capture (per-shell PID) ---
_rtlog_tmpfile=$(mktemp /tmp/.rtlog_out.XXXXXXXX)

# --- Load tools.conf ---
_rtlog_load_conf() {
    _rtlog_tools_exact=()
    _rtlog_tools_glob=()

    [[ -r "$RTLOG_CONF" ]] || return 1

    local line
    while IFS= read -r line; do
        line="${line## }"; line="${line%% }"
        line="${line##	}"; line="${line%%	}"
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
    (( ${+_rtlog_tools_exact[$1]} )) && return 0
    local pat
    for pat in "${_rtlog_tools_glob[@]}"; do
        [[ "$1" == ${~pat} ]] && return 0
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
    words=("${(z)1}")

    while (( ${#words} > 0 )); do
        case "${words[1]}" in
            *=*) shift words ;;   # skip inline env var assignments (e.g. KRB5CCNAME=ticket.ccache)
            sudo|nohup|time|env|command|exec|nice|ionice|strace|ltrace|proxychains|proxychains4|tsocks)
                shift words ;;
            *) break ;;
        esac
    done

    (( ${#words} > 0 )) || return

    local tool="${words[1]:t}"

    if ! _rtlog_match_tool "$tool"; then
        (( RTLOG_DEBUG )) && echo "[rtlog:preexec] no match for '$tool'" >&2
        return
    fi

    (( RTLOG_DEBUG )) && echo "[rtlog:preexec] matched '$tool'" >&2

    _rtlog_pending_tool="$tool"
    _rtlog_pending_cmd="$1"
    _rtlog_pending_start="$EPOCHREALTIME"
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
        command sleep 0.05 2>/dev/null
    fi
}

# --- precmd hook ---
_rtlog_precmd() {
    (( RTLOG_DEBUG )) && echo "[rtlog:precmd] pending='$_rtlog_pending_tool'" >&2
    local rc=${_rtlog_pending_rc:-$?}

    [[ -n "$_rtlog_pending_tool" ]] || return

    # Duration
    local _dur=$(( EPOCHREALTIME - _rtlog_pending_start ))
    printf -v _dur '%.1f' "$_dur"

    # Build output file argument if capture file exists
    local _out_args=()
    if [[ -n "$_rtlog_tmpfile" && -f "$_rtlog_tmpfile" ]]; then
        _out_args=(--out-file "$_rtlog_tmpfile")
    fi

    rtlog log \
        --cmd "$_rtlog_pending_cmd" \
        --tool "$_rtlog_pending_tool" \
        --exit "$rc" \
        --dur "$_dur" \
        --cwd "$PWD" \
        --tty "$_rtlog_tty" \
        "${_out_args[@]}" 2>/dev/null

    command rm -f "$_rtlog_tmpfile" 2>/dev/null

    # Reset
    _rtlog_pending_tool=""
    _rtlog_pending_cmd=""
    _rtlog_pending_start=""
}

# --- Hook registration ---
add-zsh-hook preexec _rtlog_preexec
add-zsh-hook precmd _rtlog_precmd
precmd_functions=(_rtlog_save_rc "${(@)precmd_functions}")
