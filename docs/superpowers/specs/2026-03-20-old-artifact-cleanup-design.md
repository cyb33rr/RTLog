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

These patterns are zsh-only. The python-hook and early rtlog predated bash support, so there are no equivalent bash-era old source lines to clean. The `.zshenv` file also did not have repo-based source lines (non-interactive hooks were added later, after the move to `~/.rt/`).

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

Although these old patterns are zsh-only, the removal logic applies generically in both functions. The patterns simply won't match anything in `.bashrc`, so no special casing is needed.

### Fix 3: Tag Go bin PATH export for safe identification

The original approach of resolving the current GOBIN/GOPATH at uninstall time is unsafe: the environment may have changed since setup wrote the export, causing uninstall to either miss the stale line or remove a line RTLog didn't create. The existing test `TestUninstallCleansCustomGoBinExport` explicitly asserts this conservative behavior.

Instead, tag the export line when setup writes it:

**Setup writes:**
```
export PATH="$HOME/go/bin:$PATH"  # added by rtlog
```

**Uninstall matches:** any line ending with `# added by rtlog` that contains `export PATH=`.

This way uninstall can safely identify and remove the exact line RTLog created, regardless of GOBIN/GOPATH changes between setup and uninstall.

**Migration:** Existing installs have the untagged export line. `setupShellRc()` should detect the old untagged line (matching the resolved export pattern without the tag) and replace it with the tagged version. Uninstall retains the hardcoded default removal (`export PATH="$HOME/go/bin:$PATH"` without tag) as a backward-compat fallback for installs that never ran the updated setup.

### Fix 4: Clean orphan temp files during setup

In `setupCleanup()`, after the denylist loop, glob-remove orphan temp files:
- `/tmp/.rtlog_out.*`
- `/tmp/.rtlog_ni_out.*`

This runs only during `rtlog setup` / `rtlog update`, not on every shell open. Removal errors (e.g., EPERM on files owned by other users on a shared system) are silently ignored — `os.Remove` failures are harmless here and should not interrupt setup.

### Fix 5: On-demand temp file creation in interactive hooks

Change `hook.zsh` and `hook.bash` to not pre-create the temp file at shell startup. Instead:

1. Initialize `_rtlog_tmpfile=""` (empty) at source time
2. In preexec, create via `mktemp` only when capture is needed and the var is empty
3. In precmd, delete the file and reset the var to empty

State transitions per matched command:
- Shell start: `_rtlog_tmpfile=""`
- preexec (matched tool, capture=1): `_rtlog_tmpfile=$(mktemp ...)`
- precmd: `rm -f "$_rtlog_tmpfile"; _rtlog_tmpfile=""`
- Next preexec: creates fresh `mktemp` again

This eliminates the primary leak (shells that never match a tool). A residual leak remains if a shell is killed between preexec and precmd (e.g., SIGKILL, terminal crash), but this is strictly better than the current always-leak behavior, and Fix 4 handles these residual files during the next `rtlog setup` or `rtlog update`. Together, Fix 4 + Fix 5 form the complete solution.

Non-interactive hooks are unchanged (they clean up in EXIT traps and only exist for the lifetime of the script).

## Files Changed

- `cmd/setup.go` — `setupCleanup()` denylist addition + temp file glob; `setupShellRc()` repo-based line migration + tagged export line
- `cmd/uninstall.go` — `uninstallCleanShellRc()` repo-based line removal + tagged export line matching
- `hook.zsh` — on-demand temp file creation
- `hook.bash` — on-demand temp file creation

## Testing

- Existing `cmd/setup_uninstall_test.go` covers setup/uninstall rc manipulation; extend with cases for:
  - Repo-based source line removal (both `/rtlog/hook.zsh` and `/python-hook/hook.zsh` patterns)
  - `uninstall.sh` in cleanup denylist
  - Tagged export line written by setup
  - Tagged export line removed by uninstall
  - Untagged default export line still removed by uninstall (backward compat)
  - Update existing `TestUninstallCleansCustomGoBinExport` to reflect tagged behavior
- Manual verification for hook changes (Fix 5): source `hook.zsh`, verify `_rtlog_tmpfile` is empty, run a matched tool, verify temp file is created in preexec, verify it is deleted in precmd and var reset to empty.
