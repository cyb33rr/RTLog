# RTLog UX Improvements Design

**Date:** 2026-03-20
**Status:** Draft

## Overview

Three changes to improve RTLog's user experience: engagement lifecycle management, interactive TUI editing/deletion, and removal of the standalone `edit` command.

## 1. Engagement Management Commands

### `rtlog rm <engagement>`

Delete an engagement database file.

- Validates engagement name with `logfile.ValidateEngagementName()`
- Validates engagement exists via `logfile.GetLogPath()`
- Opens the DB to get entry count, then closes it before deletion (ensures WAL checkpoint)
- Shows engagement name and entry count before confirmation
- Requires `y` at `Delete engagement "<name>" (N entries)? [y/N]` prompt
- `-y/--yes` flag skips confirmation (same pattern as `delete`/`clear`)
- If the removed engagement is the active one in state, clears the `engagement` key
- Deletes `~/.rt/logs/<engagement>.db` and any WAL sidecar files (`.db-wal`, `.db-shm`), ignoring "not found" errors on sidecars

### `rtlog rename <old> <new>`

Rename an engagement database file.

- Validates both `<old>` and `<new>` with `logfile.ValidateEngagementName()`
- Validates `<old>` exists via `logfile.GetLogPath()`
- Errors if `<new>` already exists
- Opens and closes the DB first to force WAL checkpoint (cleans up sidecar files)
- Renames `~/.rt/logs/<old>.db` to `~/.rt/logs/<new>.db`
- Also renames `-wal` and `-shm` files if they still exist (ignoring "not found")
- If `<old>` is the active engagement in state, updates state to `<new>` (preserves tag and note)

## 2. TUI Interactive Delete and Edit

Add two new keybindings to the `Selector` in `internal/display/selector.go`.

### Delete (`Ctrl+D`)

1. User presses `Ctrl+D` on a selected entry
2. Confirmation line replaces the filter bar: `Delete entry #<id>? (y/n)`
3. `y` → callback invoked, entry spliced out of `s.entries`, `s.filtered` rebuilt, cursor stays in place (or moves up if it was the last entry)
4. Any other key → cancels, restores normal filter bar

### Edit (`Ctrl+E`)

1. User presses `Ctrl+E` on a selected entry
2. Prompt replaces filter bar: `Edit: (t)ag or (n)ote?`
3. `t` → filter bar becomes `Tag: <current_value>` with cursor at end; user edits inline
4. `n` → filter bar becomes `Note: <current_value>` with cursor at end; user edits inline
5. `Enter` saves via callback; the in-memory `Entry` map's `"tag"` or `"note"` value is updated locally, entry display refreshes in-place
6. `Esc` cancels at any step, restores normal view
7. Saving an empty string clears the field (same behavior as `edit --tag ""`)

### Selector changes

The `Selector` struct needs access to the database to perform delete/update operations:

- **Callback approach:** Add `OnDelete func(id int64) error` and `OnUpdate func(id int64, fields map[string]string) error` callbacks to `Selector`. The `show` command sets these callbacks before calling `Run()`. This keeps `display` package decoupled from `db`.
- If callbacks are nil, `Ctrl+D`/`Ctrl+E` are silently ignored (no-op).
- Add `"id"` to `ToMap()` output so the Selector can map entries back to DB IDs.

### Post-mutation in-memory refresh

After **delete**: remove the entry from `s.entries` by index, rebuild `s.filtered` via `ApplyFilters()`, adjust cursor (clamp to `len(s.filtered)-1` if it overflows).

After **update**: mutate the `Entry` map in `s.entries[idx]` directly (set `"tag"` or `"note"` key), rebuild `s.filtered` in case the edit affects active filters.

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
| `internal/display/selector.go` | Add mode state machine, `Ctrl+D`/`Ctrl+E` handlers, callbacks |
| `internal/logfile/logfile.go` | Add `"id"` to `ToMap()` |
| `cmd/rm_test.go` | New — tests for rm command |
| `cmd/rename_test.go` | New — tests for rename command |

## Testing

- `cmd/rm_test.go` — Test rm with valid/invalid engagement, active engagement state update, `-y` flag, WAL cleanup
- `cmd/rename_test.go` — Test rename with valid names, name collision, active engagement state update, WAL cleanup
- TUI delete/edit — Manual testing in terminal (raw mode makes automated testing impractical for the interactive flow)
- Verify `delete` command still works standalone
- Verify `edit` command is removed (help output, direct invocation errors)
