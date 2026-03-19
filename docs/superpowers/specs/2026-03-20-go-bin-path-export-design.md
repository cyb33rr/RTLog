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

Pass the resolved Go bin dir and a flag into `setupShellRc`. Detect existing Go bin PATH exports (same approach as the `~/.local/bin` detection) and skip if already present.

### Output

- `[+]  Added ~/go/bin to PATH in .zshrc` — when export is added
- `[ok] Go bin already in PATH` — when already present

## Scope

- Only `cmd/setup.go` is modified
- No new files
- `installDefault`, `installCustom`, `installFresh` paths are untouched
- Existing `~/.local/bin` PATH logic stays as-is

## Affected code

- `runSetup()`: Pass Go bin path info into the `installGoInstall` case
- `setupShellRc()`: Add detection/insertion of Go bin PATH export
- New helper: `resolveGoBinDir(home string) (dir string, exportLine string)` to compute the directory and the portable export line
