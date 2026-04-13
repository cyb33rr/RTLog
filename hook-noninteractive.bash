# hook-noninteractive.bash - RTLog hook for non-interactive bash shells
# Sourced via BASH_ENV to capture commands from scripts, automation, etc.
# Uses DEBUG + EXIT traps (bash-preexec not needed).

# --- Skip interactive shells (handled by hook.bash via .bashrc) ---
[[ $- == *i* ]] && return 0
[[ -v _RTLOG_NI_LOADED ]] && return 0
_RTLOG_NI_LOADED=1

# --- Prevent recursive loading across exec boundaries ---
# BASH_ENV is re-sourced by every new bash process (e.g. pyenv shims,
# pyenv-exec, internal tool scripts).  The per-process _RTLOG_NI_LOADED
# guard above doesn't survive exec.  Export a flag so sub-processes skip.
[[ -n "${__RTLOG_NI_ACTIVE:-}" ]] && return 0
export __RTLOG_NI_ACTIVE=1

# --- Fast bail: no state file means rtlog isn't set up ---
_rtlog_ni_state_file="${HOME}/.rt/state"
[[ -f "$_rtlog_ni_state_file" ]] || return 0

# --- Read state ---
_rtlog_ni_engagement="" _rtlog_ni_tag="" _rtlog_ni_note=""
_rtlog_ni_enabled="1" _rtlog_ni_capture="1"

{
    key="" val=""
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
declare -gA _rtlog_ni_tools_exact
declare -ga _rtlog_ni_tools_glob
_rtlog_ni_conf="${HOME}/.rt/tools.conf"
[[ -r "$_rtlog_ni_conf" ]] || return 0

{
    line=""
    while IFS= read -r line; do
        line="${line#"${line%%[![:space:]]*}"}"
        line="${line%"${line##*[![:space:]]}"}"
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
    [[ -v "_rtlog_ni_tools_exact[$1]" ]] && return 0
    local pat
    for pat in "${_rtlog_ni_tools_glob[@]}"; do
        [[ "$1" == $pat ]] && return 0
    done
    return 1
}

# --- Pending state ---
_rtlog_ni_pending_tool=""
_rtlog_ni_pending_cmd=""
_rtlog_ni_pending_start=""
_rtlog_ni_capturing=0
_rtlog_ni_fd_out="" _rtlog_ni_fd_err=""
_rtlog_ni_outfile=""

# --- DEBUG trap handler ---
_rtlog_ni_debug_handler() {
    [[ -n "$_rtlog_ni_pending_tool" ]] && return

    local cmd="$BASH_COMMAND"
    [[ -n "$cmd" ]] || return 0

    # Don't capture our own trap/cleanup commands
    [[ "$cmd" == _rtlog_ni_* ]] && return

    local -a words
    read -ra words <<< "$cmd"

    # Strip wrappers
    while (( ${#words[@]} > 0 )); do
        case "${words[0]}" in
            *=*) words=("${words[@]:1}") ;;
            sudo|nohup|time|env|command|exec|nice|ionice|strace|ltrace|proxychains|proxychains4|tsocks)
                words=("${words[@]:1}") ;;
            *) break ;;
        esac
    done

    (( ${#words[@]} > 0 )) || return 0
    local tool="${words[0]##*/}"

    _rtlog_ni_match "$tool" || return 0

    _rtlog_ni_pending_tool="$tool"
    _rtlog_ni_pending_cmd="$cmd"
    _rtlog_ni_pending_start="$(date +%s.%N 2>/dev/null || date +%s)"

    # Output capture
    if [[ "$_rtlog_ni_capture" == "1" ]]; then
        _rtlog_ni_outfile=$(mktemp /tmp/.rtlog_ni_out.XXXXXXXX)
        exec {_rtlog_ni_fd_out}>&1 {_rtlog_ni_fd_err}>&2
        exec > >(trap - EXIT DEBUG INT TERM HUP ERR; exec tee -- "$_rtlog_ni_outfile") 2>&1
        _rtlog_ni_capturing=1
    fi
}

# --- EXIT trap handler ---
_rtlog_ni_exit_handler() {
    local rc=$?

    if (( _rtlog_ni_capturing )); then
        exec 1>&${_rtlog_ni_fd_out} 2>&${_rtlog_ni_fd_err}
        exec {_rtlog_ni_fd_out}>&- {_rtlog_ni_fd_err}>&-
        _rtlog_ni_capturing=0
        # Brief wait for tee to flush after pipe EOF
        command sleep 0.05 2>/dev/null
    fi

    if [[ -z "$_rtlog_ni_pending_tool" ]]; then
        [[ -n "$_rtlog_ni_outfile" ]] && command rm -f "$_rtlog_ni_outfile" 2>/dev/null
        return "$rc"
    fi

    # Duration (use awk for float arithmetic)
    local _dur
    _dur=$(awk "BEGIN {printf \"%.1f\", $(date +%s.%N 2>/dev/null || date +%s) - $_rtlog_ni_pending_start}")

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

    [[ -n "$_rtlog_ni_outfile" ]] && command rm -f "$_rtlog_ni_outfile" 2>/dev/null
    _rtlog_ni_outfile=""

    return "$rc"
}

# --- Register traps ---
trap '_rtlog_ni_debug_handler' DEBUG
trap '_rtlog_ni_exit_handler' EXIT
