# Old Artifact Cleanup Design

## Problem

RTLog's installation process has evolved through ~10 phases, from shell-script-based install (`install.sh`) through Go commands with multiple install cases, to the current simplified Go-only model. Each phase left different artifacts on the system. The current `setupCleanup()` and `uninstallCleanShellRc()` handle most of them, but an audit reveals 4 gaps plus a temp file leak in the shell hooks.

## Gaps Identified

### Gap 1: `~/.rt/uninstall.sh` not in cleanup denylist

The original `install.sh` (commit 8fa146c) copied `uninstall.sh` into `~/.rt/` as a runtime file. This file is not in `setupCleanup()`'s denylist, so it persists on systems that were installed with the original shell script.

### Gap 2: Repo-based source lines not cleaned

The original `install.sh` cleaned old source lines from `.zshrc`:
- `source .*/rtlog/hook.zsh` (pointing to a git checkout directory)
- `source .*/python-hook/hook.zsh` (pre-Go Python version)

Neither `setupShellRc()` nor `uninstallCleanShellRc()` handle these patterns. Users who had these lines would retain stale/broken source lines in their shell rc files.

### Gap 3: Custom GOBIN/GOPATH PATH export not cleaned by uninstall

`uninstallCleanShellRc()` only removes the exact default string `export PATH="$HOME/go/bin:$PATH"`. If setup added a PATH export for a custom GOBIN or GOPATH location (e.g., `export PATH="/opt/go/bin:$PATH"`), uninstall does not remove it.

### Gap 4: Orphan temp files in `/tmp`

Shell hooks create temp files for output capture:
- Interactive: `/tmp/.rtlog_out.XXXXXXXX` (via `mktemp`)
- Non-interactive: `/tmp/.rtlog_ni_out.XXXXXXXX` or `/tmp/.rtlog_ni_out.$$`

Interactive hooks create the temp file eagerly at shell startup. If the shell exits without ever running a matched tool, the temp file leaks. These accumulate over time.

## Design

### Fix 1: Add `uninstall.sh` to `setupCleanup()` denylist

Add `"uninstall.sh"` to the denylist array in `setupCleanup()` (`cmd/setup.go`). The file is removed on next `rtlog setup` or `rtlog update`.

### Fix 2: Remove repo-based source lines in setup and uninstall

In `setupShellRc()` and `uninstallCleanShellRc()`, add removal of lines matching:
- Contains `source` AND (`/rtlog/hook.zsh` OR `/python-hook/hook.zsh`)
- Guard: does NOT contain `.rt/hook.` (to avoid removing the current canonical source line)

This cleans old source lines from both setup migration and full uninstall paths.

### Fix 3: Resolve actual Go bin export line in uninstall

Change `uninstallCleanShellRc()` signature to accept a `goBinExportLine` parameter (the resolved export line from `resolveGoBinDir()`). Remove lines matching either:
- The hardcoded default `export PATH="$HOME/go/bin:$PATH"` (backward compat)
- The resolved `goBinExportLine` (covers custom GOBIN/GOPATH)

The caller already computes this via `resolveGoBinDir()` at uninstall.go:63.

### Fix 4: Clean orphan temp files during setup

In `setupCleanup()`, after the denylist loop, glob-remove orphan temp files:
- `/tmp/.rtlog_out.*`
- `/tmp/.rtlog_ni_out.*`

This runs only during `rtlog setup` / `rtlog update`, not on every shell open.

### Fix 5: On-demand temp file creation in interactive hooks

Change `hook.zsh` and `hook.bash` to not pre-create the temp file at shell startup. Instead:

1. Initialize `_rtlog_tmpfile=""` (empty) at source time
2. In preexec, create via `mktemp` only when capture is needed and the file doesn't exist yet
3. In precmd, delete the file and reset the var to empty

This eliminates the leak at the source — temp files only exist during the preexec-to-precmd window of a matched tool.

Non-interactive hooks are unchanged (they clean up in EXIT traps).

## Files Changed

- `cmd/setup.go` — `setupCleanup()` denylist addition + temp file glob; `setupShellRc()` repo-based line migration
- `cmd/uninstall.go` — `uninstallCleanShellRc()` signature change + repo-based line removal + dynamic Go bin export matching
- `hook.zsh` — on-demand temp file creation
- `hook.bash` — on-demand temp file creation

## Testing

- Existing `cmd/setup_uninstall_test.go` covers setup/uninstall rc manipulation; extend with cases for repo-based source lines, custom GOBIN export removal, and `uninstall.sh` cleanup.
- Manual verification: create a `.zshrc` with old-style artifacts, run `rtlog setup`, confirm they're cleaned.
