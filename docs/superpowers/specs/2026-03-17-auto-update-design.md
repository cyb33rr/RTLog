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

On every interactive command run:

1. Check if `~/.rt/last-update-check` exists and is less than 24 hours old. If so, skip.
2. If a check is needed, spawn a goroutine that makes a GET request to `https://api.github.com/repos/cyb33rr/rtlog/releases/latest` with a 3-second timeout so it never slows down the command.
3. Parse the `tag_name` field from the JSON response and compare it against the compiled-in `Version` using semver comparison.
4. If a newer version exists, write the new version string to `~/.rt/update-available`. Update `~/.rt/last-update-check` timestamp regardless.
5. After the main command finishes, if `~/.rt/update-available` exists, print to stderr: `Update available: vX.Y.Z (current: vA.B.C). Run 'rtlog update' to upgrade.`

**Exclusions:** The hidden `log` command and any non-TTY context skip the background check entirely to avoid polluting shell hook output.

**Why two files:** `last-update-check` controls throttling. `update-available` is a flag so the notification can be shown without re-checking the API.

### 2. `rtlog update` Command

Flow:

1. Fetch latest release from `https://api.github.com/repos/cyb33rr/rtlog/releases/latest`.
2. Compare `tag_name` against compiled-in `Version`. If already up to date, print message and exit.
3. **Detect install method:** Check if the current binary path is inside a `GOPATH/bin` or `GOBIN` directory.
   - If Go-installed: print `You installed via 'go install'. Run: go install github.com/cyb33rr/rtlog@latest` and exit.
   - If binary install: proceed with download.
4. **Find the right asset:** Match against release assets using the pattern `rtlog-{GOOS}-{GOARCH}` (derived from `runtime.GOOS` and `runtime.GOARCH`). If no matching asset found, error out.
5. **Download** the asset to a temp file in the same directory as the current binary (same filesystem for atomic rename).
6. **Verify:** Check that the downloaded file is a valid executable (non-zero size, correct ELF/Mach-O header bytes as a basic sanity check).
7. **Replace:** `chmod` the temp file to match the current binary's permissions, then `os.Rename` the temp file over the current binary path.
8. **Cleanup:** Remove `~/.rt/update-available` if it exists. Update `~/.rt/last-update-check`.
9. Print success: `Updated rtlog: vA.B.C → vX.Y.Z`

### 3. Version Comparison

- Strip the `v` prefix from both the compiled-in `Version` and the GitHub `tag_name` (e.g., `v1.2.0` → `1.2.0`).
- Split on `.` and compare major, minor, patch as integers.
- If `Version` is `dev` (local build without tags), always consider it outdated and offer the update.
- No external semver library needed — rtlog uses simple `vX.Y.Z` tags.

### 4. File Layout & Integration

**New files:**
- `internal/update/update.go` — Core logic: version check, download, self-replace, Go-install detection.
- `cmd/update.go` — Cobra command wiring for `rtlog update`.

**Modified files:**
- `cmd/root.go` — Add post-run hook for background version check notification on interactive commands. Skip for `log` command and non-TTY.

**State files (in `~/.rt/`):**
- `last-update-check` — Contains the Unix timestamp of the last API check.
- `update-available` — Contains the latest version string when an update is available. Deleted after a successful update or when the check finds no update.

**No new dependencies.** Uses only standard library packages: `net/http`, `encoding/json`, `os`, `runtime`, `path/filepath`, `strconv`, `strings`, `time`.

### 5. Edge Cases & Safety

- **Permissions:** If the binary location isn't writable, the rename fails. Print a clear error: `Permission denied. Try: sudo rtlog update` or suggest re-downloading manually.
- **Concurrent runs:** If two shells trigger a background check simultaneously, the writes are idempotent (same version string, same timestamp) — no conflict.
- **No internet / API rate limit:** The 3-second timeout on background checks means the command isn't delayed. If the check fails silently, the user just doesn't get notified. The `rtlog update` command prints the actual error.
- **GitHub API rate limit:** Unauthenticated requests are limited to 60/hour. With once-per-day checks, this is a non-issue.
- **Partial download:** The temp-file + rename pattern ensures the old binary is intact until the download is fully complete and verified.
- **Architecture mismatch:** If no matching asset is found in the release, print available assets and suggest downloading manually.
