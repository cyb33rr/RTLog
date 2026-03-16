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
    zmodload -F zsh/datetime p:EPOCHREALTIME
    local _dur=$(( EPOCHREALTIME - _rtlog_ni_pending_start ))
    printf -v _dur '%.1f' "$_dur"

    # Build output file argument if capture file exists
    local _out_args=()
    if [[ -n "$_rtlog_ni_outfile" && -f "$_rtlog_ni_outfile" ]]; then
        _out_args=(--out-file "$_rtlog_ni_outfile")
    fi

    rtlog log \
        --cmd "$_rtlog_ni_pending_cmd" \
        --tool "$_rtlog_ni_pending_tool" \
        --exit "$rc" \
        --dur "$_dur" \
        --cwd "$PWD" \
        "${_out_args[@]}" 2>/dev/null

    command rm -f "$_rtlog_ni_outfile" 2>/dev/null
}

# --- Register traps ---
trap '_rtlog_ni_debug_handler' DEBUG
trap '_rtlog_ni_exit_handler' EXIT
