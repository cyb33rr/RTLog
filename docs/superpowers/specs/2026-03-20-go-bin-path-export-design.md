# Go Bin PATH Export in Setup

**Date:** 2026-03-20
**Status:** Approved

## Problem

After `go install github.com/cyb33rr/rtlog@latest`, the binary lands in Go's bin directory (`$GOBIN`, `$GOPATH/bin`, or `~/go/bin`). If that directory isn't in the user's PATH, `rtlog setup` detects it (because the current shell has it), but doesn't ensure future shells will find it. The `installGoInstall` case skips PATH configuration entirely.

## Solution

In the `installGoInstall` case, resolve the Go bin directory and add a PATH export to the user's shell rc files if not already present. This follows the exact same pattern setup already uses for `~/.local/bin` in the fresh-install case.

## Design

### Resolve Go bin directory

Use the same logic `isGoInstalled` already uses:
1. If `$GOBIN` is set, use it
2. Else if `$GOPATH` is set, use `$GOPATH/bin`
3. Else use `~/go/bin` (Go's default)

### Portable export line

- Default case (`~/go/bin`): `export PATH="$HOME/go/bin:$PATH"`
- Custom `$GOPATH`: `export PATH="<gopath>/bin:$PATH"`
- Custom `$GOBIN`: `export PATH="<gobin>:$PATH"`

Use `$HOME` instead of hardcoded home path for portability when the path is under home.

### Integration with setupShellRc

Add a new `goBinExportLine string` parameter to `setupShellRc`. When non-empty, the function scans for and inserts the Go bin PATH export. This is mutually exclusive with `addPathExport` (the `~/.local/bin` flag) — `installGoInstall` always sets `addPathExport = false`.

Detection of "already present": match lines where the non-commented, trimmed line equals the exact export line produced by `resolveGoBinDir`. This is the same literal-match approach used for `~/.local/bin`. If the user added Go bin to PATH through other means (e.g., system-level `/etc/profile.d/`), a redundant export may be added — this is harmless and accepted.

### Uninstall cleanup

`uninstallCleanShellRc` in `cmd/uninstall.go` must also remove the default Go bin PATH export line:
- `export PATH="$HOME/go/bin:$PATH"`

Custom GOBIN/GOPATH paths are not removed at uninstall time because there is no persistent state recording what setup originally wrote, and the user's environment may have changed. Removing only the well-known default avoids accidentally deleting user-configured exports.

### Output

- `[+]  Added ~/go/bin to PATH in .zshrc` — when export is added
- `[ok] Go bin already in PATH` — when already present

## Scope

- `cmd/setup.go` — main changes
- `cmd/uninstall.go` — cleanup of Go bin PATH exports
- `installDefault`, `installCustom`, `installFresh` paths are untouched
- Existing `~/.local/bin` PATH logic stays as-is

## Affected code

- `runSetup()`: Call `resolveGoBinDir` in the `installGoInstall` case, pass result to `setupShellRc`
- `setupShellRc()`: New `goBinExportLine string` parameter; detect/insert Go bin PATH export
- `uninstallCleanShellRc()`: Match and remove Go bin PATH export lines
- New helper: `resolveGoBinDir(home string) (dir string, exportLine string)` to compute the directory and the portable export line. If path is not under `$HOME`, uses absolute path directly.
