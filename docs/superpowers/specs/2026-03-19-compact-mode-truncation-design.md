# Compact Mode: Command Truncation & Right-Aligned Metadata

**Date:** 2026-03-19
**Status:** Approved

## Problem

In the Atuin-style TUI (`FmtCompact`), long commands push metadata fields (`[tag]`, `# note`, `[+out]`) off the right edge of the terminal, making them invisible. The tag and note fields are important operational context that operators need to see at a glance.

## Solution

Truncate the command to fit the available terminal width, right-align metadata at the terminal edge, and cap note text at 15 characters.

## Layout Structure

Each compact line is composed of three zones:

```
<timestamp>  <command><padding><metadata>
|  fixed  |  |  flexible  |  | measured |
```

- **Timestamp**: always rendered as 10 chars — either `HH:MM:SS` (8 chars) + 2-space gap, or 10 spaces when the timestamp is empty. This keeps the zone fixed regardless of data.
- **Command**: variable width, truncated with `…` when too long
- **Metadata**: right-aligned at terminal edge; width varies per entry (exit code width, duration digits, optional fields) — measured at render time
- **Gutter**: minimum 2 spaces between command and metadata. This replaces the existing 2-space separator between command and exit code in the format string.

Width budget:

```
metadataWidth = visibleLen(metadataSuffix)
commandWidth  = terminalWidth - 10(timestamp zone) - 2(gutter) - metadataWidth
```

## Metadata Format

Metadata is assembled as a single string. Each optional field, when present, includes its own leading 2-space separator. Absent fields contribute 0 characters (including 0 separator). The mandatory fields (exit code, duration) are always present and separated by 2 spaces.

```
exit:N  Ns  [tag]  # note  [+out]
```

- `exit:N` — always present, variable width (e.g., `exit:0` = 6 chars, `exit:127` = 8 chars)
- `Ns` — always present, variable width (e.g., `0s` = 2 chars, `123.456s` = 8 chars)
- `[tag]` — optional, prefixed with `  ` when present
- `# note` — optional, prefixed with `  ` when present
- `[+out]` — optional, prefixed with `  ` when present

Metadata visible width is computed using `visibleLen()` which strips ANSI color codes and counts runes.

## Note Truncation

- If note text exceeds 15 visible characters, truncate to 14 + `…`
- The 15-char cap applies to the note text only, not the `# ` prefix
- Example: `# this is a very…`

## Command Truncation

- **Fits**: left-align after timestamp, pad with spaces to push metadata to right edge
- **Too long**: truncate to `(commandWidth - 1)` chars + `…`, then gutter padding, then metadata
- **Narrow terminal**: if `commandWidth` < 10, clamp to 10. Metadata may be partially cut by the existing `truncateVisible` in the render loop — acceptable for degraded narrow terminals

`truncateText` operates on plain text only. Callers must not pass ANSI-colored strings. In the implementation, command text is truncated before any colorization is applied (commands are not colorized in `FmtCompact`).

## Expanded Mode

When the user presses Enter to expand a row in the TUI, the selected row displays the **truncated** command (same `FmtCompact` output), not the full command. The full command can be inferred from the captured output context. Since `FmtCompact` now guarantees its output fits within `width`, `wrapExtra` in the render loop will always be 0, and the existing wrap logic can remain but will effectively be a no-op.

## Visual Examples

```
14:32:01  nmap -sV -sC -p- 10.10.10.1        exit:0  12s  [recon]  # initial scan  [+out]
14:33:15  gobuster dir -u http://10.10.10…    exit:0   8s  [recon]
14:35:02  curl -s http://10.10.10.1/api/v…    exit:1   2s           # interesting
```

## Known Limitations

- **Wide characters (CJK, emoji)**: `visibleLen` counts runes, not terminal columns. East Asian characters and emoji occupy 2 terminal columns per rune. This will cause misalignment if present in commands or notes. Acceptable for a pentesting tool where commands are predominantly ASCII.

## Code Changes

### `internal/display/format.go`

- `FmtCompact(entry Entry)` → `FmtCompact(entry Entry, width int)`
- Add `visibleLen(s string) int` — counts runes excluding ANSI escape sequences. Uses the same ANSI-stripping approach as existing `RE_ANSI.ReplaceAllString` + `len([]rune(...))` to avoid divergence with other call sites.
- Add `truncateText(s string, max int) string` — truncates plain text (no ANSI) with `…` when exceeding max length
- Build metadata string first, measure its visible width
- Truncate note to 15 chars, truncate command to remaining space
- Pad between command and metadata to right-align

### `internal/display/selector.go`

- `render()` passes terminal width `w` to `FmtCompact(entry, w)`
- Expanded-mode plain text calculation at lines 340-341 also passes width
- `wrapExtra` logic remains but becomes effectively a no-op since `FmtCompact` output now fits `width`

### `internal/display/format_test.go`

- Update all 4 existing `FmtCompact` tests to pass a width argument
- Add new tests:
  - Command truncation (long command gets `…`)
  - Note truncation (note > 15 chars gets `…`)
  - Narrow terminal clamping (width < minimum)
  - Short command with right-aligned padding
  - Empty timestamp (verify 10-char fixed zone)
