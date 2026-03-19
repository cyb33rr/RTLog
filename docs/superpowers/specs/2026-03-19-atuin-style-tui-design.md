# Atuin-Style TUI for RTLog

## Overview

Enhance RTLog's interactive `show` command with an Atuin-style TUI: compact colored rows, inline text filtering, and preset toggle filters. Replaces the current menu-based selector with a filter-first browsing experience while keeping zero new dependencies.

## Layout

Newest entry at the bottom, older entries above. Filter bar pinned below the list.

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│  14:12:01  nmap -sC -sV 10.10.14.0/24  exit:0  12s  [recon]  # initial scan  [+out] │
│  14:15:44  crackmapexec smb 10.10.14.0/24  exit:0  3s  [recon]                      │
│  14:18:07  gobuster dir -u http://10.10.14.5 -w ...  exit:0  8s  [recon]  [+out]    │
│▸ 14:22:01  nmap -sV -p 1-1000 10.10.14.5  exit:0  8.1s  [recon]  # port scan       │  ← selected
│                                                                                      │
│  [recon] [!fail]  4/42 matches   ▸ nmap_                                            │  ← filter bar
└──────────────────────────────────────────────────────────────────────────────────────┘
```

Each entry is a single line. Long commands are truncated to terminal width. The diagram is wide for clarity; in practice, entries truncate to fit.

- Cursor starts at the bottom (newest entry).
- Up arrow moves to older entries, down arrow to newer.

## Row Format

All entries use a single compact format with colors:

```
HH:MM:SS  <command>  exit:N  Ns  [tag]  # note  [+out]
```

- **Timestamp**: `HH:MM:SS`
- **Command**: full command line, newlines collapsed to spaces
- **Exit code**: green if 0, red otherwise
- **Duration**: dim
- **Tag**: yellow, omitted if empty
- **Note**: omitted if empty
- **`[+out]`**: dim indicator, omitted if no captured output

No index number. No separate tool name (visible in the command itself).

The selected row is highlighted with inverted colors. No auto-expanded metadata line.

`Enter` toggles captured output display below the selected entry (same scrollable output view as the current selector).

## Keybindings

### Navigation

| Key | Action |
|-----|--------|
| `↑` | Move cursor to older entry (or scroll output when expanded) |
| `↓` | Move cursor to newer entry (or scroll output when expanded) |
| `Enter` | Toggle captured output for selected entry |
| `Esc` | Quit |

### Filtering

| Key | Action |
|-----|--------|
| Any printable character | Append to text filter |
| `Backspace` | Remove last character from filter (rune-aware) |
| `Tab` | Cycle tag filter: all → recon → exploitation → ... → all |
| `Ctrl+F` | Toggle failed-only (non-zero exit codes) |

`Esc` is the only quit key. The current `q`-to-quit and `r`-to-reverse keybindings are removed — `q` and `r` now type into the filter like any other printable character. The `r`-to-reverse feature is removed entirely (display order is always newest-at-bottom). All other keys either filter or navigate.

## Filtering Behavior

- **Text filter**: case-insensitive substring match across command, tool, tag, note, and CWD fields.
- **Tag filter**: exact match on the tag field. Cycles through tags that actually exist in the current engagement's data.
- **Failed-only**: filters to entries with non-zero exit codes.
- **Stacking**: all filters combine (AND logic). Typing `nmap` with `[recon]` active shows only nmap commands tagged recon.
- **Instant**: filters re-apply on every keystroke. In-memory filtering over a slice is fast enough for thousands of entries.
- **Cursor reset**: when filter changes, cursor resets to the bottom (newest match).

## Filter Bar

Pinned at the bottom, always visible.

**With active filters:**
```
  [recon] [!fail]  5/42 matches   ▸ nmap_
```

**No filters active:**
```
  42 entries   ▸ _
```

**Components (left to right):**
- **Tag badge**: current tag filter, highlighted yellow when active, hidden when "all"
- **Fail badge**: `[!fail]` highlighted red when active, hidden when off
- **Match count**: `filtered/total matches` when filters active, `total entries` when not
- **Text input**: `▸` prompt followed by filter text

**No matches:**
```
  [recon] [!fail]  0/42 matches   ▸ nmap_
```
The list area shows a dim centered `(no matches)` message.

## Implementation

### Files Changed

#### `internal/display/selector.go`

Evolve the existing `Selector` struct:

- **New fields:**
  - `filter string` — current text filter
  - `tagFilter string` — current tag filter ("" = all)
  - `failOnly bool` — failed-only toggle
  - `filtered []int` — indices into original `entries` slice that match current filters
  - `allTags []string` — unique tags from the data, for Tab cycling
  - `tagIdx int` — current position in the tag cycle

- **Input handling:**
  - Replace single-key-only handling with mixed mode
  - Printable characters (0x20–0x7E, plus UTF-8 sequences) append to `filter`
  - `Backspace` (0x7F or 0x08 — handle both for terminal compatibility) removes last rune from `filter`
  - Arrow keys, Enter, Tab, Ctrl+F keep their navigation/toggle roles
  - `Esc` (0x1B, single byte — not part of an escape sequence) quits. Disambiguation: the existing approach of reading up to 3 bytes per `Read()` call works — escape sequences arrive as a burst, so a lone 0x1B means Esc.

- **New methods:**
  - `applyFilters()` — rebuilds `filtered` slice by testing each entry against text filter, tag filter, and fail-only. Called on any filter state change.
  - `collectTags()` — scans all entries (not just filtered) to build `allTags` slice for Tab cycling. Called once at startup. Tags are engagement-level, not filter-dependent.
  - `renderFilterBar(width int) string` — renders the bottom filter bar.

- **Display order:**
  - Entries stored in chronological order (oldest first, as returned by DB)
  - Rendered bottom-up: newest entry draws at the bottom of the visible area
  - Cursor starts at the last entry (newest)

#### `internal/display/format.go`

Add `FmtCompact()` function:

```
func FmtCompact(entry Entry) string
```

Format: `HH:MM:SS  cmd  exit:N  Ns  [tag]  # note  [+out]`

The `[+out]` indicator is determined by checking `entry["out"]` internally — entry maps always carry the `"out"` key (as an empty string when no output was captured), so no parameter is needed.

Reuses existing helpers: `formatTimestamp`, `getString`, `getInt`, `getFloat`, `Colorize`.

#### `cmd/show.go`

- Remove the manual entry reversal (selector handles display order).
- Remove the engagement name header passed to the selector (the filter bar replaces it as the primary chrome).
- No new flags.
- Non-interactive path (`--all` flag, non-TTY) unchanged — continues to use `FmtEntry()` with index numbers and tool names.

### Files Not Changed

- `internal/display/color.go` — no changes needed
- `internal/db/db.go` — no changes needed
- `internal/logfile/` — no changes needed
- Shell hooks — no changes needed
- Export functionality — no changes needed

### No New Dependencies

Everything is built with the existing raw terminal mode infrastructure and ANSI escape code handling already in `selector.go` and `color.go`.

## Edge Cases

- **Empty engagement**: shows `(no entries)` centered, filter bar visible but input has no effect.
- **All filtered out**: shows `(no matches)` centered, filter bar active so user can adjust.
- **Long commands**: truncated to terminal width via existing `truncateVisible()`.
- **Terminal resize**: re-renders on next keypress (no SIGWINCH handler needed).
- **No tags in data**: `Tab` does nothing, tag badge stays hidden.
- **Single entry**: cursor stays put, expand/collapse works normally.
- **Unicode in filter**: backspace removes last rune, not last byte.

## Testing

- **Unit tests for `applyFilters()`**: text match, tag filter, fail-only, combinations, empty filter, no matches.
- **Unit tests for `FmtCompact()`**: all field combinations, empty fields, color codes.
- **Integration test**: `rtlog show` with test DB, verify non-interactive output still works unchanged.

No TUI interaction tests — raw mode is impractical to test programmatically; visual verification is more effective.
