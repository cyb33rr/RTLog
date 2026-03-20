# Manual Install Support

**Date:** 2026-03-20
**Status:** Draft

## Problem

RTLog's setup, update, and uninstall commands assume the binary was installed via `go install`. Users who build from source with `make build && ./rtlog setup` hit several issues:

1. **Setup** unconditionally injects a Go bin PATH export into shell rc files — useless or wrong for manual installs
2. **Update** unconditionally runs `go install` — fails without a Go toolchain or when the user wants to stay on a local branch
3. **Hooks** call `rtlog` by name — silently broken if the user hasn't added the binary to PATH yet

## Design

### 1. Shared helper: `isGoInstall() bool`

Runtime detection of install method. No persistent state.

- Resolve `os.Executable()`, follow symlinks via `filepath.EvalSymlinks`
- Determine Go bin directory: `GOBIN` > `$GOPATH/bin` > `~/go/bin`
- Return `true` if the executable path starts with the resolved Go bin directory

Lives in `cmd/` alongside the existing `resolveGoBinDir` helper.

### 2. Setup changes

**Remove Go bin PATH export logic:**

- Remove the `resolveGoBinDir` call from `setupCore`
- Remove the `goBinExportLine` parameter from `setupShellRc`
- Remove all code in `setupShellRc` that checks for, adds, or migrates Go bin PATH export lines

**Add legacy PATH line cleanup to `setupShellRc`:**

- Remove lines matching `# added by rtlog` tag (the old tagged PATH exports)
- Remove untagged `export PATH="$HOME/go/bin:$PATH"` lines (backward compat)
- Remove `export PATH="$HOME/.local/bin:$PATH"` lines (already handled)
- This runs as part of the existing line-scanning loop, before the source line check

**Add post-setup PATH warning:**

- After all setup steps complete (end of `setupCore`), call `exec.LookPath("rtlog")`
- If not found, print:
  ```
  [!]  Warning: rtlog is not on your PATH — hooks won't work until you add it.
  ```

**Order of operations in `setupShellRc`:**

1. Scan lines, removing legacy source lines and legacy PATH export lines
2. Add hook source line if missing
3. (Go bin PATH export is no longer added)

**Post-setup in `setupCore`:**

1. Steps 1-8 as today (minus Go bin PATH)
2. Remove legacy PATH lines (done inside `setupShellRc`)
3. Check `exec.LookPath("rtlog")` and warn if not found

**Update command description** in `setupCmd` to remove references to Go toolchain requirement and Go bin PATH.

### 3. Update changes

In `runUpdate`, before running `go install`:

- Call `isGoInstall()`
- If `false`: print `"rtlog was not installed via go install — update manually with git pull && make build"` and return without error
- If `true`: proceed with `go install` + `setupCore` as today

Update command description to reflect this behavior.

### 4. Uninstall changes

- **No functional changes** — uninstall already removes legacy `# added by rtlog` PATH lines and untagged Go bin PATH lines
- Update the binary location advice (step 4) to use `isGoInstall()`:
  - If go-installed: advise `rm <go-bin>/rtlog` as today
  - If manual: advise user to remove the binary from wherever they placed it (print the resolved `os.Executable()` path)

## Files changed

| File | Change |
|------|--------|
| `cmd/setup.go` | Remove `resolveGoBinDir`, remove `goBinExportLine` from `setupShellRc`, add legacy PATH cleanup, add PATH warning |
| `cmd/update.go` | Add `isGoInstall()` gate before `go install` |
| `cmd/uninstall.go` | Use `isGoInstall()` for binary location advice |
| `cmd/helpers.go` (new or existing) | Add `isGoInstall()` helper |
| `cmd/setup_uninstall_test.go` | Update tests for removed PATH export logic, add tests for `isGoInstall()` and PATH warning |

## What does NOT change

- Makefile — no `make install` target; users handle binary placement
- Hook files — no changes to hook.zsh/hook.bash
- Shell rc source lines — hook sourcing stays as-is
- PATH management — users are responsible for their own PATH
- Uninstall legacy cleanup — keeps removing old `# added by rtlog` lines
