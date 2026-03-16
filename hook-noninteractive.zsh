# hook-noninteractive.zsh - RTLog hook for non-interactive zsh shells
# Sourced via ~/.zshenv to capture commands from Claude Code, scripts, etc.
# Uses DEBUG + EXIT traps instead of preexec/precmd (which don't fire in
# non-interactive mode).

# --- Skip interactive shells (handled by hook.zsh via .zshrc) ---
[[ -o interactive ]] && return 0
(( ${+_RTLOG_NI_LOADED} )) && return 0
_RTLOG_NI_LOADED=1

# --- Fast bail: no state file means rtlog isn't set up ---
_rtlog_ni_state_file="${HOME}/.rt/state"
[[ -f "$_rtlog_ni_state_file" ]] || return 0

# --- Read state ---
typeset -g _rtlog_ni_engagement="" _rtlog_ni_tag="" _rtlog_ni_note=""
typeset -g _rtlog_ni_enabled="1" _rtlog_ni_capture="1"

{
    local key val
    while IFS='=' read -r key val; do
        case "$key" in
            engagement) _rtlog_ni_engagement="$val" ;;
            tag)        _rtlog_ni_tag="$val" ;;
            note)       _rtlog_ni_note="$val" ;;
            enabled)    _rtlog_ni_enabled="$val" ;;
            capture)    _rtlog_ni_capture="$val" ;;
        esac
    done < "$_rtlog_ni_state_file"
}

# --- Bail if disabled or no engagement ---
[[ "$_rtlog_ni_enabled" == "1" ]] || return 0
[[ -n "$_rtlog_ni_engagement" ]] || return 0

# --- Load tools.conf ---
typeset -gA _rtlog_ni_tools_exact
typeset -ga _rtlog_ni_tools_glob
_rtlog_ni_conf="${HOME}/.rt/tools.conf"
[[ -r "$_rtlog_ni_conf" ]] || return 0

{
    local line
    while IFS= read -r line; do
        line="${line## }"; line="${line%% }"
        line="${line##	}"; line="${line%%	}"
        [[ -z "$line" || "$line" == \#* ]] && continue
        if [[ "$line" == *'*'* || "$line" == *'?'* ]]; then
            _rtlog_ni_tools_glob+=("$line")
        else
            _rtlog_ni_tools_exact[$line]=1
        fi
    done < "$_rtlog_ni_conf"
}

# --- Tool matching ---
_rtlog_ni_match() {
    (( ${+_rtlog_ni_tools_exact[$1]} )) && return 0
    local pat
    for pat in "${_rtlog_ni_tools_glob[@]}"; do
        [[ "$1" == ${~pat} ]] && return 0
    done
    return 1
}

# --- Pending state ---
typeset -g _rtlog_ni_pending_tool=""
typeset -g _rtlog_ni_pending_cmd=""
typeset -g _rtlog_ni_pending_start=""
typeset -g _rtlog_ni_capturing=0
typeset -g _rtlog_ni_fd_out="" _rtlog_ni_fd_err=""
typeset -g _rtlog_ni_outfile="/tmp/.rtlog_ni_out.$$"

# --- JSON escape (same as hook.zsh) ---
_rtlog_ni_json_escape() {
    local s="$1"
    s="${s//\\/\\\\}"
    s="${s//\"/\\\"}"
    s="${s//$'\n'/\\n}"
    s="${s//$'\t'/\\t}"
    s="${s//$'\r'/\\r}"
    s="${s//$'\x00'/\\u0000}"; s="${s//$'\x01'/\\u0001}"
    s="${s//$'\x02'/\\u0002}"; s="${s//$'\x03'/\\u0003}"
    s="${s//$'\x04'/\\u0004}"; s="${s//$'\x05'/\\u0005}"
    s="${s//$'\x06'/\\u0006}"; s="${s//$'\x07'/\\u0007}"
    s="${s//$'\x08'/\\u0008}"; s="${s//$'\x0b'/\\u000b}"
    s="${s//$'\x0c'/\\u000c}"; s="${s//$'\x0e'/\\u000e}"
    s="${s//$'\x0f'/\\u000f}"; s="${s//$'\x10'/\\u0010}"
    s="${s//$'\x11'/\\u0011}"; s="${s//$'\x12'/\\u0012}"
    s="${s//$'\x13'/\\u0013}"; s="${s//$'\x14'/\\u0014}"
    s="${s//$'\x15'/\\u0015}"; s="${s//$'\x16'/\\u0016}"
    s="${s//$'\x17'/\\u0017}"; s="${s//$'\x18'/\\u0018}"
    s="${s//$'\x19'/\\u0019}"; s="${s//$'\x1a'/\\u001a}"
    s="${s//$'\x1b'/\\u001b}"; s="${s//$'\x1c'/\\u001c}"
    s="${s//$'\x1d'/\\u001d}"; s="${s//$'\x1e'/\\u001e}"
    s="${s//$'\x1f'/\\u001f}"
    REPLY="$s"
}

# --- DEBUG trap handler (fires BEFORE each command) ---
_rtlog_ni_debug_handler() {
    # Only capture the first matched tool per shell invocation
    [[ -n "$_rtlog_ni_pending_tool" ]] && return

    local cmd="$ZSH_DEBUG_CMD"
    [[ -n "$cmd" ]] || return

    # Parse command to extract tool
    local -a words
    words=("${(z)cmd}")

    # Strip wrappers
    while (( ${#words} > 0 )); do
        case "${words[1]}" in
            *=*) shift words ;;
            sudo|nohup|time|env|command|exec|nice|ionice|strace|ltrace|proxychains|proxychains4|tsocks)
                shift words ;;
            *) break ;;
        esac
    done

    (( ${#words} > 0 )) || return
    local tool="${words[1]:t}"

    _rtlog_ni_match "$tool" || return

    # Matched — record pending state
    zmodload -F zsh/datetime p:EPOCHREALTIME
    _rtlog_ni_pending_tool="$tool"
    _rtlog_ni_pending_cmd="$cmd"
    _rtlog_ni_pending_start="$EPOCHREALTIME"

    # Output capture
    if [[ "$_rtlog_ni_capture" == "1" ]]; then
        : > "$_rtlog_ni_outfile"
        exec {_rtlog_ni_fd_out}>&1 {_rtlog_ni_fd_err}>&2
        exec > >(tee -- "$_rtlog_ni_outfile") 2>&1
        _rtlog_ni_capturing=1
    fi
}

# --- EXIT trap handler (fires when shell exits) ---
_rtlog_ni_exit_handler() {
    local rc=$?

    # Restore FDs — closing the pipe sends EOF to tee, causing it to flush
    if (( _rtlog_ni_capturing )); then
        exec 1>&${_rtlog_ni_fd_out} 2>&${_rtlog_ni_fd_err}
        exec {_rtlog_ni_fd_out}>&- {_rtlog_ni_fd_err}>&-
        _rtlog_ni_capturing=0
        # Brief wait for tee's process substitution subshell to finish
        # flushing after receiving EOF. Without this, the output file
        # may be empty/truncated for fast commands.
        command sleep 0.05 2>/dev/null
    fi

    [[ -n "$_rtlog_ni_pending_tool" ]] || return

    # Duration
    zmodload -F zsh/datetime p:EPOCHREALTIME p:EPOCHSECONDS b:strftime
    local dur=$(( EPOCHREALTIME - _rtlog_ni_pending_start ))
    printf -v dur '%.1f' "$dur"

    # Timestamp
    local ts
    strftime -s ts '%Y-%m-%dT%H:%M:%SZ' "$EPOCHSECONDS"
    local epoch=$EPOCHSECONDS

    # Escape fields
    _rtlog_ni_json_escape "$_rtlog_ni_pending_cmd"; local escaped_cmd="$REPLY"
    _rtlog_ni_json_escape "$_rtlog_ni_note";        local escaped_note="$REPLY"
    _rtlog_ni_json_escape "$USER";                  local escaped_user="$REPLY"
    _rtlog_ni_json_escape "${HOST:-$(hostname)}";   local escaped_host="$REPLY"
    _rtlog_ni_json_escape "$PWD";                   local escaped_cwd="$REPLY"
    _rtlog_ni_json_escape "$_rtlog_ni_tag";         local escaped_tag="$REPLY"

    # Read captured output
    local escaped_out=""
    if [[ -s "$_rtlog_ni_outfile" ]]; then
        local raw_out="$(<"$_rtlog_ni_outfile")"
        _rtlog_ni_json_escape "$raw_out"
        escaped_out="$REPLY"
    fi
    command rm -f "$_rtlog_ni_outfile" 2>/dev/null

    # Write JSONL entry
    local logdir="${RTLOG_DIR:-$HOME/.rt/logs}"
    [[ -d "$logdir" ]] || command mkdir -p "$logdir"

    printf '{"ts":"%s","epoch":%d,"user":"%s","host":"%s","tty":"%s","cwd":"%s","tool":"%s","cmd":"%s","exit":%d,"dur":%s,"tag":"%s","note":"%s","out":"%s"}\n' \
        "$ts" "$epoch" "$escaped_user" "$escaped_host" "noninteractive" "$escaped_cwd" \
        "$_rtlog_ni_pending_tool" "$escaped_cmd" "$rc" "$dur" "$escaped_tag" "$escaped_note" "$escaped_out" \
        >> "$logdir/${_rtlog_ni_engagement}.jsonl"

    # Clear one-shot note
    if [[ -n "$_rtlog_ni_note" ]]; then
        local tmp="${_rtlog_ni_state_file}.tmp"
        printf 'engagement=%s\ntag=%s\nnote=%s\nenabled=%s\ncapture=%s\n' \
            "$_rtlog_ni_engagement" "$_rtlog_ni_tag" "" "$_rtlog_ni_enabled" "$_rtlog_ni_capture" > "$tmp"
        command mv -f "$tmp" "$_rtlog_ni_state_file"
    fi
}

# --- Register traps ---
trap '_rtlog_ni_debug_handler' DEBUG
trap '_rtlog_ni_exit_handler' EXIT
