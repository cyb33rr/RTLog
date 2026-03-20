# RTLog UX Improvements Design

**Date:** 2026-03-20
**Status:** Draft

## Overview

Three changes to improve RTLog's user experience: engagement lifecycle management, interactive TUI editing/deletion, and removal of the standalone `edit` command.

## 1. Engagement Management Commands

### `rtlog rm <engagement>`

Delete an engagement database file.

- Validates engagement exists via `logfile.GetLogPath()`
- Shows engagement name and entry count before confirmation
- Requires `y` at `Delete engagement "<name>" (N entries)? [y/N]` prompt
- `-y/--yes` flag skips confirmation (same pattern as `delete`/`clear`)
- If the removed engagement is the active one in state, clears the `engagement` key
- Deletes `~/.rt/logs/<engagement>.db`

### `rtlog rename <old> <new>`

Rename an engagement database file.

- Validates `<old>` exists via `logfile.GetLogPath()`
- Validates `<new>` with `logfile.ValidateEngagementName()`
- Errors if `<new>` already exists
- Renames `~/.rt/logs/<old>.db` to `~/.rt/logs/<new>.db`
- If `<old>` is the active engagement in state, updates state to `<new>`

## 2. TUI Interactive Delete and Edit

Add two new keybindings to the `Selector` in `internal/display/selector.go`.

### Delete (`d` key)

1. User presses `d` on a selected entry
2. Confirmation line replaces the filter bar: `Delete entry #<id>? (y/n)`
3. `y` → entry is deleted from DB, entry list refreshes, cursor adjusts (stays in place, or moves up if it was the last entry)
4. Any other key → cancels, restores normal filter bar

### Edit (`e` key)

1. User presses `e` on a selected entry
2. Prompt replaces filter bar: `Edit: (t)ag or (n)ote?`
3. `t` → filter bar becomes `Tag: <current_value>` with cursor at end; user edits inline
4. `n` → filter bar becomes `Note: <current_value>` with cursor at end; user edits inline
5. `Enter` saves the new value via `db.Update()`; entry refreshes in-place
6. `Esc` cancels at any step, restores normal view

### Selector changes

The `Selector` struct needs access to the database to perform delete/update operations. Options:

- **Callback approach (recommended):** Add `OnDelete func(id int64) error` and `OnUpdate func(id int64, fields map[string]string) error` callbacks to `Selector`. The `show` command sets these callbacks before calling `Run()`. This keeps `display` package decoupled from `db`.
- The Selector also needs a way to map displayed entries back to their DB IDs. Currently entries are `map[string]interface{}` (the `Entry` type). The `id` field is not present in `ToMap()`. Add `"id"` to `ToMap()` output.

### Selector state machine

Add a `mode` field to `Selector`:

```
type selectorMode int
const (
    modeNormal selectorMode = iota
    modeConfirmDelete
    modeEditChoose    // waiting for t/n
    modeEditTag       // editing tag value
    modeEditNote      // editing note value
)
```

Each mode changes:
- What the filter bar displays (prompt text)
- What keystrokes do (mode-specific handlers)
- `Esc` always returns to `modeNormal`

### Key conflict check

- `d` and `e` currently fall through to the printable ASCII handler (typing into the filter). They will be intercepted in `modeNormal` only when the entry list is non-empty and an entry is selected. This means typing `d` or `e` into the search filter is no longer possible.
- **Mitigation:** Users can still match entries containing `d`/`e` via other characters in the search term, or use regex mode. This is an acceptable trade-off since `d` and `e` as single-character search terms have very low utility.

## 3. Remove Standalone `edit` Command

- Delete `cmd/edit.go`
- Remove `editCmd` registration from root command
- The `delete` command stays — it's useful for scripting/non-interactive use

## Files Changed

| File | Change |
|------|--------|
| `cmd/rm.go` | New — `rtlog rm` command |
| `cmd/rename.go` | New — `rtlog rename` command |
| `cmd/edit.go` | Deleted |
| `cmd/show.go` | Pass delete/update callbacks to Selector |
| `internal/display/selector.go` | Add mode state machine, `d`/`e` handlers, callbacks |
| `internal/logfile/logfile.go` | Add `"id"` to `ToMap()` |

## Testing

- `cmd/rm_test.go` — Test rm with valid/invalid engagement, active engagement state update, `-y` flag
- `cmd/rename_test.go` — Test rename with valid names, name collision, active engagement state update
- TUI delete/edit — Manual testing in terminal (raw mode makes automated testing impractical for the interactive flow)
- Verify `delete` command still works standalone
- Verify `edit` command is removed (help output, direct invocation errors)
