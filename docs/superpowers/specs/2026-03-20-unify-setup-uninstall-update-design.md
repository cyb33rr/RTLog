# Unify Setup, Uninstall, and Update Around Go Install

**Date:** 2026-03-20
**Status:** Approved

## Problem

RTLog can only be installed via `go install` (or building from source), which always places the binary in Go's bin directory. Yet setup supports four install cases (goInstall, custom, default/symlink, fresh) that assume the binary could be anywhere. This creates three issues:

1. `rtlog update` runs `go install`, which always puts the new binary in Go's bin dir — but if the user originally set up via the default/symlink path, the old binary at `~/.rt/rtlog` remains, and which one runs depends on PATH ordering.
2. Uninstall doesn't know about all install locations, so it can't reliably clean up.
3. Dead code paths (symlink creation, binary copying, `~/.local/bin` management) add complexity for cases that can't actually occur.

## Solution

Simplify all three commands around a single assumption: **Go is required, the binary lives in Go's bin directory.** Setup migrates old installs, update calls setup after `go install`, and uninstall shares path resolution with setup.

## Design

### Setup (simplified)

Eight idempotent steps:

1. **Create directories** — `~/.rt/logs/`
2. **Cleanup stale files** — existing denylist, plus `~/.rt/rtlog` binary (migration)
3. **Migrate old installs** — remove `~/.local/bin/rtlog` if it is a symlink pointing to `~/.rt/rtlog`. Non-matching symlinks and regular files at that path are left alone. Remove `export PATH="$HOME/.local/bin:$PATH"` from rc files. The `~/.local/bin/` directory itself is not removed (it may be used by other tools).
4. **Write embedded files** — hooks and config files (unchanged behavior)
5. **Resolve Go bin dir** — `resolveGoBinDir()` finds canonical Go bin path. This function always returns a path (defaulting to `~/go/bin` when neither `$GOBIN` nor `$GOPATH` is set) and cannot fail.
6. **Configure shell rc files** — add hook source line + Go bin PATH export if not already present
7. **Configure `.zshenv`** — non-interactive zsh hook
8. **Configure `BASH_ENV`** — non-interactive bash hook

**Removed from setup:**
- `detectBinaryPath()` and the 4 install cases (`installGoInstall`, `installCustom`, `installDefault`, `installFresh`)
- `setupCopySelfTo()` — atomic binary copy helper
- `setupSymlink()` — symlink creation helper
- `~/.local/bin/` directory creation
- `addPathExport` flag for `~/.local/bin` PATH export (replaced by Go bin export only)

### Update

Two steps:

1. **Run `go install github.com/cyb33rr/rtlog@latest`**
2. **Call setup's core logic** — re-runs full idempotent setup so hooks/config match the new version

If `go install` fails, setup is not run.

**Implementation note:** `runSetup` currently uses `Run` (calls `os.Exit` on failure) while `runUpdate` uses `RunE` (returns errors). To allow update to call setup, extract setup's core logic into a helper function that returns errors. Both `runSetup` and `runUpdate` call this helper — `runSetup` converts errors to `os.Exit`, `runUpdate` returns them.

### Uninstall

Four steps:

1. **Clean shell rc files** — remove hook source lines, Go bin PATH export (default `$HOME/go/bin` pattern only), old `~/.local/bin` PATH export (`export PATH="$HOME/.local/bin:$PATH"`), `BASH_ENV` export, non-interactive hook lines, and associated comments. Collapse consecutive blank lines. The `~/.local/bin` removal is needed for users who run uninstall directly on an old install without first running the new setup.
2. **Clean `.zshenv`** — remove non-interactive zsh hook + comment
3. **Remove `~/.rt/` directory** — prompt user unless `-y` flag
4. **Advise on binary removal** — resolve Go bin dir via `resolveGoBinDir()`, print `rm <path>/rtlog`

**Removed from uninstall:**
- Symlink removal step (no more symlinks)
- Duplicated Go install detection logic in `uninstallAdviseGoInstall()` (uses shared `resolveGoBinDir` instead)

**Preserved behavior:** only removes the default Go bin PATH pattern (`$HOME/go/bin`). Custom `$GOBIN`/`$GOPATH` paths are preserved since we can't know if the user set those for other tools.

### Shared code

All three commands share these helpers:

- **`resolveGoBinDir(home, gopath, gobin)`** — single source of truth for Go bin directory and portable export line. Used by setup (step 5) and uninstall (step 4, advise message).
- **`isGoInstalled(binPath, gopath, gobin)`** — no longer needed. Uninstall's advise step uses `resolveGoBinDir` directly to get the path, rather than checking whether the current binary is Go-installed.
- **`collapseBlankLines()`** — rc file cleanup
- **`fileExists()`** — utility

**Removed:**
- `detectBinaryPath()` — no longer needed
- `setupCopySelfTo()` — no longer needed
- `setupSymlink()` — no longer needed
- `isGoInstalled()` — replaced by `resolveGoBinDir` everywhere
- Duplicated Go detection logic in `uninstallAdviseGoInstall()`

## Migration

Users who previously installed via the old default path (`~/.rt/rtlog` + `~/.local/bin/rtlog` symlink) are migrated automatically on next `rtlog setup` or `rtlog update`:

1. `~/.rt/rtlog` binary is deleted by cleanup denylist
2. `~/.local/bin/rtlog` symlink is removed if it points to `~/.rt/rtlog`
3. `export PATH="$HOME/.local/bin:$PATH"` is removed from rc files
4. Go bin PATH export is added in its place

## Affected code

- `cmd/setup.go` — extract core logic into a helper that returns errors; simplify to Go-bin-only path; add migration steps; remove dead install cases. `setupShellRc` signature simplified: remove `localBin` and `addPathExport` parameters (only `goBinExportLine` remains for PATH management).
- `cmd/update.go` — call setup's core helper after `go install`
- `cmd/uninstall.go` — remove symlink step; add `~/.local/bin` PATH export removal; use `resolveGoBinDir` for advise; remove `isGoInstalled` and duplicated Go detection
- `cmd/setup_uninstall_test.go` — update tests

### Key test scenarios

- **Migration: old symlink removed** — `~/.local/bin/rtlog` symlink pointing to `~/.rt/rtlog` is removed
- **Migration: non-matching symlink preserved** — `~/.local/bin/rtlog` pointing elsewhere is left alone
- **Migration: regular file preserved** — `~/.local/bin/rtlog` as a regular file is left alone
- **Migration: old PATH export removed** — `export PATH="$HOME/.local/bin:$PATH"` removed from rc files
- **Migration: old binary cleaned** — `~/.rt/rtlog` deleted by cleanup denylist
- **Uninstall on old install** — `~/.local/bin` PATH export removed even without prior new setup
- **Update calls setup** — after `go install`, setup core logic runs and configures rc files
- **Update fails on go install** — setup is not run when `go install` fails
