# Setup Improvements: Smart Binary Path Detection & Cleanup

## Summary

Improve `rtlog setup` to (1) detect an existing binary on PATH and install there instead of always writing to `~/.rt/rtlog`, and (2) clean up stale files from previous versions on every setup run while preserving user data.

## Motivation

Users may install `rtlog` to custom locations (e.g., `/usr/local/bin/`, `/opt/tools/`). The current setup always copies to `~/.rt/rtlog`, creating a duplicate binary when one already exists on PATH at a different location. Additionally, there is no mechanism to clean up outdated hooks or state files from previous versions during upgrades.

## Design

### 1. Smart Binary Path Detection

During setup, before the binary copy step, detect where `rtlog` currently lives:

**Detection logic (in order):**

1. Use `exec.LookPath("rtlog")` to find the binary on PATH
2. Resolve symlinks via `filepath.EvalSymlinks` to get the real path

**Four cases (evaluated in order):**

| Case | Condition | Action |
|------|-----------|--------|
| Go install | Found on PATH, resolved path is inside `GOPATH/bin` or `GOBIN` | Skip binary copy. Print `go install github.com/cyb33rr/rtlog@latest` |
| Custom path | Found on PATH, real path is NOT `~/.rt/rtlog` | Copy binary to that location. Skip `~/.local/bin/` creation, symlink, and PATH export in rc files. On permission error, print advisory (e.g., `Try: sudo rtlog setup`) instead of fatal exit |
| Default path | Found on PATH, real path IS `~/.rt/rtlog` (directly or via symlink) | Current behavior: copy to `~/.rt/rtlog`, create `~/.local/bin/`, ensure symlink |
| Fresh install | Not found on PATH | Current behavior: create `~/.local/bin/`, copy to `~/.rt/rtlog`, create symlink, add PATH export |

**Edge cases:**
- Running binary is the one being replaced: use atomic temp+rename (already implemented)
- `exec.LookPath` could theoretically find a different binary named `rtlog` that isn't ours. This is a known limitation — in practice negligible for a tool named `rtlog`
- `~/.local/bin/` is only created in the "default path" and "fresh install" cases

### 2. Setup Cleanup (Denylist)

On every `rtlog setup` run, before writing embedded files, delete the following files if they exist:

**Denylist (deleted and re-created fresh):**
- `~/.rt/hook.zsh`
- `~/.rt/hook.bash`
- `~/.rt/hook-noninteractive.zsh`
- `~/.rt/hook-noninteractive.bash`
- `~/.rt/bash-preexec.sh`
- `~/.rt/last-update-check`
- `~/.rt/update-available`

These are all application-managed files that get re-written from embedded defaults by the subsequent setup steps. 

**Not on denylist (handled separately):**
- `~/.rt/rtlog` — managed by the binary installation step (Section 1)

**Implementation note:** Delete each denylist entry individually using `os.Remove()`. Do not use glob patterns or directory listing.

**Preserved (never deleted):**
- `~/.rt/logs/` — engagement databases (user data)
- `~/.rt/state` — active engagement, tag, enabled/capture flags
- `~/.rt/tools.conf` — diff against embedded default, prompt user before overwriting if different
- `~/.rt/extract.conf` — diff against embedded default, prompt user before overwriting if different

### 3. Updated Setup Flow

```
rtlog setup:
  1. Create directories (~/.rt/logs/)
  2. Cleanup: delete denylist files from ~/.rt/
  3. Write embedded files:
     - Hooks and bash-preexec: written fresh (always)
     - tools.conf, extract.conf: diff + prompt if modified by user
  4. Binary installation:
     a. Use exec.LookPath("rtlog") + EvalSymlinks to find existing binary
     b. If go install path → skip copy, print go install command
     c. If custom path → copy there, skip ~/.local/bin/ and symlink
     d. If default path → copy to ~/.rt/rtlog, create ~/.local/bin/, ensure symlink
     e. If not found → create ~/.local/bin/, copy, symlink, PATH export
  5. Configure shell rc files (~/.zshrc, ~/.bashrc):
     - Hook source line
     - PATH export (only if step 4e)
     - BASH_ENV export
  6. Configure ~/.zshenv for non-interactive zsh capture
```

### 4. Changes to Existing Code

**`cmd/setup.go`:**
- Add `setupCleanup(rtDir)` function with the denylist
- Modify `runSetup` to call cleanup before writing embedded files
- Refactor binary installation logic:
  - Replace `isOnPath()` with `detectBinaryPath()` that returns the resolved path and an enum/type indicating the case (goinstall/custom/default/fresh)
  - `detectBinaryPath()` calls `update.IsGoInstalled()` to identify the go install case, using the same `GOPATH`/`GOBIN` resolution pattern as `cmd/update.go`
  - Note: this is an intentional behavioral change from `isOnPath()` which uses `os.Executable()` (the running binary) — `detectBinaryPath()` uses `exec.LookPath()` (what PATH resolves to)
  - Conditionally skip `~/.local/bin/` creation, symlink, and PATH export based on the case
- Update `setupShellRc` to accept a flag for whether PATH export is needed

**No changes to:**
- `internal/state/` — state file format and handling unchanged
- `internal/update/` — update command unchanged (shares `IsGoInstalled` from `internal/update/`)
- `internal/db/` — database layer unchanged
- Shell hooks — content unchanged, just re-written fresh

**Follow-up needed:**
- `cmd/uninstall.go` — currently assumes binary is at `~/.rt/rtlog` and symlink at `~/.local/bin/rtlog`. After this change, uninstall will not know about custom-path installations. This should be addressed in a follow-up task.

## Testing

- Test `detectBinaryPath()` for each case:
  - `TestDetectBinaryPath_GoInstall` — binary found in `GOPATH/bin`
  - `TestDetectBinaryPath_CustomPath` — binary found at `/usr/local/bin/rtlog`
  - `TestDetectBinaryPath_DefaultPath` — binary found at `~/.rt/rtlog` (via symlink or direct)
  - `TestDetectBinaryPath_FreshInstall` — binary not found on PATH
- Test `setupCleanup()` correctly deletes denylist files and preserves others
- Test that `tools.conf`/`extract.conf` prompt behavior is preserved
- Test fresh install path (no existing binary)
- Test upgrade path (existing binary at custom location)
- Test upgrade path (existing binary at default `~/.rt/` location)
- Test `go install` case — detected correctly, binary copy skipped
- Test custom path with permission error — advisory printed, not fatal
