# rtlog Auto-Update Feature Design

## Overview

Add a self-update mechanism to rtlog with two components:
1. **Background version check** — silently checks GitHub Releases once per day on interactive commands and notifies the user if a new version is available.
2. **`rtlog update` command** — manually triggered command that downloads and installs the latest version, replacing the current binary in-place.

## Distribution Context

rtlog is distributed via two channels:
- `go install github.com/cyb33rr/rtlog@latest` (requires Go toolchain)
- Pre-built binaries uploaded to GitHub Releases (linux/darwin, amd64/arm64)

The update mechanism must handle both install methods appropriately.

## Design

### 1. Background Version Check

**Synchronization model:** The background check writes state files for the *next* invocation to pick up. The current invocation only displays a notification if `~/.rt/update-available` was already written by a *previous* run. This avoids any goroutine synchronization complexity.

On every interactive command run:

1. If `~/.rt/update-available` exists, print to stderr: `Update available: vX.Y.Z (current: vA.B.C). Run 'rtlog update' to upgrade.` This uses the file written by a previous invocation.
2. Check if `~/.rt/last-update-check` exists and is less than 24 hours old. If so, skip the API call.
3. If a check is needed, spawn a fire-and-forget goroutine in `PersistentPreRunE` (before the command runs, giving it maximum time) that:
   - Makes a GET request to `https://api.github.com/repos/cyb33rr/rtlog/releases/latest` with a 3-second timeout.
   - Parses the `tag_name` field and compares it against the compiled-in `Version`.
   - If a newer version exists, writes the new version string to `~/.rt/update-available`.
   - If the latest version matches the running version, removes `~/.rt/update-available` to clear stale notifications.
   - Updates `~/.rt/last-update-check` timestamp regardless.
   - Creates `~/.rt/` directory if it doesn't exist (following the `os.MkdirAll` pattern used in `state.go`).
4. The goroutine is fire-and-forget — if the process exits before it completes, the check is simply skipped. No `sync.WaitGroup` needed.

**Exclusions:** The `log` command, the `update` command, and any non-TTY context skip the background check entirely.

**Why two files:** `last-update-check` controls throttling. `update-available` is a flag so the notification can be shown without re-checking the API.

**Opt-out:** Setting `RTLOG_NO_UPDATE_CHECK=1` disables the background check entirely (for air-gapped environments or organizational policies).

### 2. `rtlog update` Command

Flow:

1. Fetch latest release from `https://api.github.com/repos/cyb33rr/rtlog/releases/latest`.
2. Compare `tag_name` against compiled-in `Version`. If already up to date and `--force` is not set, print message and exit.
3. **Detect install method:** Resolve the current binary path using `os.Executable()` followed by `filepath.EvalSymlinks()` (matching the pattern in `setup.go`). Check if the resolved path is inside `GOPATH/bin` or `GOBIN`.
   - If Go-installed: print `You installed via 'go install'. Run: go install github.com/cyb33rr/rtlog@latest` and exit.
   - If binary install: proceed with download.
4. **Find the right asset:** Match against release assets using the pattern `rtlog-{GOOS}-{GOARCH}` (derived from `runtime.GOOS` and `runtime.GOARCH`). If no matching asset found, print available assets and suggest manual download.
5. **Download** the asset to a temp file in the same directory as the resolved binary path (same filesystem for atomic rename).
6. **Verify:** Check that the downloaded file is a valid executable (non-zero size, correct ELF/Mach-O header bytes as a basic sanity check). Note: integrity relies on HTTPS to GitHub's API and CDN; no additional checksum verification is performed.
7. **Replace:** `chmod` the temp file to match the current binary's permissions, then `os.Rename` the temp file over the resolved binary path. The symlink at `~/.local/bin/rtlog` is preserved since only the target file is replaced.
8. **Cleanup:** Remove `~/.rt/update-available` if it exists. Update `~/.rt/last-update-check`.
9. Print success: `Updated rtlog: vA.B.C → vX.Y.Z`

**Flags:**
- `--force` — Re-download and replace even if the current version matches the latest (useful for corrupted binaries).

**Note on self-replacement:** On Linux/macOS (the only targets), replacing a running binary via `os.Rename` is safe because the kernel holds a reference to the open inode. The old binary continues executing until the process exits.

### 3. Version Comparison

- Strip the `v` prefix from both the compiled-in `Version` and the GitHub `tag_name` (e.g., `v1.2.0` → `1.2.0`).
- Split on `.` and compare major, minor, patch as integers.
- If `Version` is `dev` (local build without tags), skip the background check entirely — developers building from source should not be nagged to downgrade to a release. The `rtlog update` command still works for dev builds but prints a warning: `Current version is 'dev' (local build). Update will replace with latest release. Continue? [y/N]`
- No external semver library needed — rtlog uses simple `vX.Y.Z` tags.

### 4. File Layout & Integration

**New files:**
- `internal/update/update.go` — Core logic: version check, download, self-replace, Go-install detection, version comparison.
- `cmd/update.go` — Cobra command wiring for `rtlog update`.

**Modified files:**
- `cmd/root.go` — Append background check goroutine launch to the existing `PersistentPreRunE` (which currently handles extraction config auto-creation). Add notification display in `PersistentPostRunE` on `rootCmd`; future subcommands must not override it without calling the parent. Skip for `log`, `update` commands and non-TTY.

**State files (in `~/.rt/`):**
- `last-update-check` — Contains the Unix timestamp of the last API check, stored as a decimal ASCII string (e.g., `1710672000`).
- `update-available` — Contains the latest version string when an update is available. Deleted after a successful update or when the background check finds the user is up to date.

**No new dependencies.** Uses only standard library packages: `net/http`, `encoding/json`, `os`, `runtime`, `path/filepath`, `strconv`, `strings`, `time`, `fmt`.

### 5. Edge Cases & Safety

- **Permissions:** If the binary location isn't writable, the rename fails. Print a clear error: `Permission denied. Try: sudo rtlog update` or suggest re-downloading manually.
- **Symlinks:** `os.Executable()` + `filepath.EvalSymlinks()` resolves to the real binary path. The symlink at `~/.local/bin/rtlog` is untouched — only the target file at `~/.rt/rtlog` (or wherever it resolves to) is replaced.
- **Concurrent runs:** If two shells trigger a background check simultaneously, the writes are idempotent (same version string, same timestamp) — no conflict.
- **No internet / API rate limit:** The 3-second timeout on background checks means the command isn't delayed. If the check fails silently, the user just doesn't get notified. The `rtlog update` command prints the actual error.
- **GitHub API rate limit:** Unauthenticated requests are limited to 60/hour. With once-per-day checks, this is a non-issue. Standard `HTTP_PROXY`/`HTTPS_PROXY` environment variables are respected via Go's default HTTP transport.
- **Partial download:** The temp-file + rename pattern ensures the old binary is intact until the download is fully complete and verified.
- **Architecture mismatch:** If no matching asset is found in the release, print available assets and suggest downloading manually.
- **Missing `~/.rt/` directory:** Update logic creates `~/.rt/` via `os.MkdirAll(dir, 0700)` before writing state files, matching the pattern in `state.go`.
- **Dev builds:** Background check is skipped entirely when `Version == "dev"` to avoid nagging developers.
