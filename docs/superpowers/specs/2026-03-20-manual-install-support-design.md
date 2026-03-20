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
- Determine Go bin directory via `resolveGoBinDir` (reuse existing helper — do NOT duplicate)
- Resolve the Go bin directory through `filepath.EvalSymlinks` as well (handles symlinked `~/go` etc.)
- Return `true` if the executable path starts with the resolved Go bin directory
- On any error from `os.Executable()` or `filepath.EvalSymlinks`, return `false` (safe default — treats as manual install)

Lives in `cmd/helpers.go` (new file). `resolveGoBinDir` stays in `cmd/setup.go` — it is still needed by uninstall and `isGoInstall()`.

**Known limitation:** If the user changes `GOBIN`/`GOPATH` after `go install`, the runtime check may return `false` (false negative). The fallback behavior (manual update message) is not harmful.

### 2. Setup changes

**Remove Go bin PATH export logic from `setupCore`:**

- Remove the `resolveGoBinDir` call from `setupCore` (line 101)
- Remove the `goBinExportLine` parameter from `setupShellRc` — signature becomes `setupShellRc(rcFile, hookFile, rcName string)`
- Update both call sites in `setupCore` (lines 108, 113)

**Simplify `setupShellRc` line-scanning loop:**

- Remove all code that checks for, adds, or migrates Go bin PATH export lines (the `hasGoBinExport` variable, the `untagged` variable, the tagged/untagged checks, the append block)
- **Convert existing migration removals to legacy cleanup** using hardcoded pattern matching (mirroring `uninstallCleanShellRc` lines 122-129):
  - Tagged lines: remove any line where `strings.Contains(trimmed, "export PATH=") && strings.HasSuffix(trimmed, rtlogTag)`
  - Untagged default: remove lines matching literal `export PATH="$HOME/go/bin:$PATH"`
  - The existing `export PATH="$HOME/.local/bin:$PATH"` removal stays
- Keep the legacy source line removal (`isLegacySourceLine`)
- Keep the hook source line check and append

**Add post-setup PATH warning in `runSetup` (NOT in `setupCore`):**

- After `setupCore` returns successfully, call `exec.LookPath("rtlog")`
- If not found, print:
  ```
  [!]  Warning: rtlog is not on your PATH — hooks won't work until you add it.
  ```
- This lives in `runSetup` to avoid spurious warnings during `rtlog update` (where the binary is already on PATH via Go bin)

**Update `setupCmd` descriptions:**

- Remove "Requires Go toolchain (binary installed via 'go install')." from `Long`
- Remove step 5 ("Resolve Go bin directory and ensure it is on PATH")
- Remove Go bin PATH mention from step 6
- Renumber remaining steps (1-7 instead of 1-8)

### 3. Update changes

In `runUpdate`, before running `go install`:

- Call `isGoInstall()`
- If `false`: print `"rtlog was not installed via go install — update manually with git pull && make build"` and return `nil`
- If `true`: proceed with `go install` + `setupCore` as today

Update `updateCmd.Long` to mention that manual installs are detected and advised.

### 4. Uninstall changes

**`uninstallCleanShellRc` — no functional changes needed.** It already handles legacy cleanup via:
- Suffix matching for tagged lines: `strings.HasSuffix(trimmed, rtlogTag)` (line 122)
- Hardcoded fallback: `export PATH="$HOME/go/bin:$PATH"` (line 126)
- These catch all historical PATH export variants without needing the exact `goBinExportLine`

The `goBinExportLine` parameter to `uninstallCleanShellRc` is redundant but harmless — leave it for now to minimize churn.

**Binary location advice (lines 66-68):** Use `isGoInstall()`:
- If go-installed: advise `rm <go-bin>/rtlog` as today
- If manual: print the resolved `os.Executable()` path and advise user to remove it

### 5. Test changes

Tests in `cmd/setup_uninstall_test.go` affected by the `setupShellRc` signature change:

| Test | Change |
|------|--------|
| `TestSetupShellRcGoBinExport` (line 314) | Delete — tests removed functionality |
| `TestSetupShellRcGoBinExportAlreadyPresent` (line 332) | Delete — tests removed functionality |
| `TestSetupShellRcNoGoBinExport` (line 352) | Delete — tests removed functionality |
| `TestSetupShellRcMigratesLocalBinExport` (line 572) | Update — remove `goBinExportLine` param, verify line is removed (not replaced) |
| `TestSetupShellRcMigratesUntaggedExport` (line 635) | Convert — verify untagged Go bin export is removed entirely |
| `TestResolveGoBinDir` (line 428) | Keep — `resolveGoBinDir` is still used by uninstall and `isGoInstall()` |

**New tests to add:**

- `TestIsGoInstall` — binary in Go bin dir returns `true`, binary elsewhere returns `false`
- `TestSetupShellRcRemovesTaggedExport` — tagged `# added by rtlog` PATH lines are removed during setup
- `TestSetupShellRcRemovesUntaggedGoBinExport` — untagged `export PATH="$HOME/go/bin:$PATH"` is removed
- `TestUpdateSkipsManualInstall` — `isGoInstall()` false path prints warning and returns
- `TestRunSetupPathWarning` — warning printed when `rtlog` not on PATH

## Files changed

| File | Change |
|------|--------|
| `cmd/setup.go` | Remove `goBinExportLine` from `setupCore` and `setupShellRc`, convert Go bin export logic to removal-only, update `setupCmd` descriptions |
| `cmd/update.go` | Add `isGoInstall()` gate before `go install` |
| `cmd/uninstall.go` | Use `isGoInstall()` for binary location advice |
| `cmd/helpers.go` (new) | Add `isGoInstall()` helper |
| `cmd/setup_uninstall_test.go` | Delete 3 tests, update 2 tests, keep `TestResolveGoBinDir`, add 5 new tests |

## What does NOT change

- `resolveGoBinDir` — stays in `cmd/setup.go`, used by uninstall and `isGoInstall()`
- Makefile — no `make install` target; users handle binary placement
- Hook files — no changes to hook.zsh/hook.bash
- Shell rc source lines — hook sourcing stays as-is
- PATH management — users are responsible for their own PATH
- Uninstall legacy cleanup — keeps removing old `# added by rtlog` lines
- `uninstallCleanShellRc` signature — left as-is to minimize churn
