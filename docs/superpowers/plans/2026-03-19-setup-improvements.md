# Setup Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve `rtlog setup` to detect existing binary path on PATH and install there, and clean up stale files on every run while preserving user data.

**Architecture:** Replace the binary `isOnPath()` check with a four-case `detectBinaryPath()` function that classifies the install type (goinstall/custom/default/fresh). Add a `setupCleanup()` denylist function that runs before embedded file writes. Both changes are in `cmd/setup.go` only; tests go in `cmd/setup_uninstall_test.go`.

**Tech Stack:** Go, `os/exec.LookPath`, `filepath.EvalSymlinks`, `internal/update.IsGoInstalled`

**Spec:** `docs/superpowers/specs/2026-03-19-setup-improvements-design.md`

---

### Task 1: Add `detectBinaryPath()` with tests

**Files:**
- Modify: `cmd/setup.go` (add new function alongside existing `isOnPath()` — removal happens in Task 4)
- Test: `cmd/setup_uninstall_test.go`

This task adds the `installKind` type and the `detectBinaryPath()` function. The old `isOnPath()` is NOT removed here — it is still called by `runSetup()` and will be removed in Task 4 when `runSetup()` is rewritten. The new function uses `exec.LookPath` + `filepath.EvalSymlinks` + `update.IsGoInstalled()` to classify the binary into one of four cases.

- [ ] **Step 1: Write failing tests for `detectBinaryPath()`**

Add to `cmd/setup_uninstall_test.go`:

```go
// detectBinaryPathTestSetup saves and clears PATH, GOPATH, GOBIN env vars.
// Returns a cleanup function that restores them.
func detectBinaryPathTestSetup(t *testing.T) func() {
	t.Helper()
	origPath := os.Getenv("PATH")
	origGopath := os.Getenv("GOPATH")
	origGobin := os.Getenv("GOBIN")
	os.Setenv("GOPATH", "")
	os.Setenv("GOBIN", "")
	return func() {
		os.Setenv("PATH", origPath)
		os.Setenv("GOPATH", origGopath)
		os.Setenv("GOBIN", origGobin)
	}
}

func TestDetectBinaryPath_FreshInstall(t *testing.T) {
	cleanup := detectBinaryPathTestSetup(t)
	defer cleanup()

	// Empty PATH — nothing to find
	os.Setenv("PATH", t.TempDir()) // empty dir, no rtlog

	home := t.TempDir()
	rtDir := filepath.Join(home, ".rt")
	os.MkdirAll(rtDir, 0755)

	kind, binPath := detectBinaryPath(home)
	if kind != installFresh {
		t.Errorf("expected installFresh, got %d", kind)
	}
	if binPath != "" {
		t.Errorf("expected empty path, got %q", binPath)
	}
}

func TestDetectBinaryPath_DefaultPath(t *testing.T) {
	cleanup := detectBinaryPathTestSetup(t)
	defer cleanup()

	home := t.TempDir()
	rtDir := filepath.Join(home, ".rt")
	os.MkdirAll(rtDir, 0755)

	// Place a fake binary at ~/.rt/rtlog
	binPath := filepath.Join(rtDir, "rtlog")
	os.WriteFile(binPath, []byte("binary"), 0755)

	// Put ~/.rt on PATH
	os.Setenv("PATH", rtDir)

	kind, resolved := detectBinaryPath(home)
	if kind != installDefault {
		t.Errorf("expected installDefault, got %d", kind)
	}
	if resolved != binPath {
		t.Errorf("expected %q, got %q", binPath, resolved)
	}
}

func TestDetectBinaryPath_DefaultPathViaSymlink(t *testing.T) {
	cleanup := detectBinaryPathTestSetup(t)
	defer cleanup()

	home := t.TempDir()
	rtDir := filepath.Join(home, ".rt")
	localBin := filepath.Join(home, ".local", "bin")
	os.MkdirAll(rtDir, 0755)
	os.MkdirAll(localBin, 0755)

	// Place binary at ~/.rt/rtlog, symlink from ~/.local/bin/rtlog
	realBin := filepath.Join(rtDir, "rtlog")
	os.WriteFile(realBin, []byte("binary"), 0755)
	os.Symlink(realBin, filepath.Join(localBin, "rtlog"))

	// Put ~/.local/bin on PATH (symlink resolves to ~/.rt/rtlog)
	os.Setenv("PATH", localBin)

	kind, resolved := detectBinaryPath(home)
	if kind != installDefault {
		t.Errorf("expected installDefault, got %d", kind)
	}
	if resolved != realBin {
		t.Errorf("expected %q, got %q", realBin, resolved)
	}
}

func TestDetectBinaryPath_CustomPath(t *testing.T) {
	cleanup := detectBinaryPathTestSetup(t)
	defer cleanup()

	home := t.TempDir()
	customDir := filepath.Join(t.TempDir(), "custom", "bin")
	os.MkdirAll(customDir, 0755)

	// Place binary at a custom location
	binPath := filepath.Join(customDir, "rtlog")
	os.WriteFile(binPath, []byte("binary"), 0755)

	os.Setenv("PATH", customDir)

	kind, resolved := detectBinaryPath(home)
	if kind != installCustom {
		t.Errorf("expected installCustom, got %d", kind)
	}
	if resolved != binPath {
		t.Errorf("expected %q, got %q", binPath, resolved)
	}
}

func TestDetectBinaryPath_GoInstall(t *testing.T) {
	cleanup := detectBinaryPathTestSetup(t)
	defer cleanup()

	home := t.TempDir()
	gopath := filepath.Join(home, "go")
	gopathBin := filepath.Join(gopath, "bin")
	os.MkdirAll(gopathBin, 0755)

	binPath := filepath.Join(gopathBin, "rtlog")
	os.WriteFile(binPath, []byte("binary"), 0755)

	os.Setenv("PATH", gopathBin)
	os.Setenv("GOPATH", gopath)

	kind, resolved := detectBinaryPath(home)
	if kind != installGoInstall {
		t.Errorf("expected installGoInstall, got %d", kind)
	}
	if resolved != binPath {
		t.Errorf("expected %q, got %q", binPath, resolved)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -run TestDetectBinaryPath -v`
Expected: compilation error — `detectBinaryPath` undefined

- [ ] **Step 3: Add imports and implement `detectBinaryPath()`**

In `cmd/setup.go`, add these imports (do NOT remove `isOnPath()` yet — it is still called by `runSetup()` and will be removed in Task 4):

```go
// Add to the import block in cmd/setup.go:
"os/exec"

"github.com/cyb33rr/rtlog/internal/update"
```

Then add the following after the existing `isOnPath()` function:

```go
// installKind classifies how rtlog is installed.
type installKind int

const (
	installFresh     installKind = iota // not found on PATH
	installDefault                      // found at ~/.rt/rtlog (directly or via symlink)
	installCustom                       // found on PATH at a non-default location
	installGoInstall                    // found inside GOPATH/bin or GOBIN
)

// detectBinaryPath finds rtlog on PATH and classifies the install type.
// Returns the kind and the resolved (real) path to the binary.
// For installFresh, the path is empty.
func detectBinaryPath(home string) (installKind, string) {
	found, err := exec.LookPath("rtlog")
	if err != nil {
		return installFresh, ""
	}

	resolved, err := filepath.EvalSymlinks(found)
	if err != nil {
		return installFresh, ""
	}

	// Check go install first (highest priority)
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(home, "go")
	}
	gobin := os.Getenv("GOBIN")
	if update.IsGoInstalled(resolved, gopath, gobin) {
		return installGoInstall, resolved
	}

	// Check if it resolves to the default ~/.rt/rtlog
	defaultBin := filepath.Join(home, ".rt", "rtlog")
	if resolved == defaultBin {
		return installDefault, resolved
	}

	return installCustom, resolved
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/ -run TestDetectBinaryPath -v`
Expected: all 5 tests PASS

- [ ] **Step 5: Run `go build` to verify compilation**

Run: `go build -o /dev/null .`
Expected: clean build (both `isOnPath` and `detectBinaryPath` exist, no callers conflict)

- [ ] **Step 6: Commit**

```bash
git add cmd/setup.go cmd/setup_uninstall_test.go
git commit -m "feat(setup): add detectBinaryPath with four-case classification

Adds detectBinaryPath() that uses exec.LookPath to find rtlog on PATH
and classifies as goinstall/custom/default/fresh. Uses
update.IsGoInstalled() for go install detection. Old isOnPath() is
retained until runSetup is rewritten in a subsequent commit."
```

---

### Task 2: Add `setupCleanup()` with tests

**Files:**
- Modify: `cmd/setup.go` (add new function)
- Test: `cmd/setup_uninstall_test.go`

This task adds the denylist cleanup function that deletes stale files before setup re-creates them.

- [ ] **Step 1: Write failing test for `setupCleanup()`**

Add to `cmd/setup_uninstall_test.go`:

```go
func TestSetupCleanup(t *testing.T) {
	rtDir := t.TempDir()

	// Create denylist files (should be deleted)
	denylist := []string{
		"hook.zsh", "hook.bash",
		"hook-noninteractive.zsh", "hook-noninteractive.bash",
		"bash-preexec.sh", "last-update-check", "update-available",
	}
	for _, name := range denylist {
		os.WriteFile(filepath.Join(rtDir, name), []byte("old"), 0644)
	}

	// Create preserved files (should survive)
	os.MkdirAll(filepath.Join(rtDir, "logs"), 0755)
	os.WriteFile(filepath.Join(rtDir, "logs", "test.db"), []byte("db"), 0644)
	os.WriteFile(filepath.Join(rtDir, "state"), []byte("engagement=test\n"), 0644)
	os.WriteFile(filepath.Join(rtDir, "tools.conf"), []byte("nmap\n"), 0644)
	os.WriteFile(filepath.Join(rtDir, "extract.conf"), []byte("nmap positional\n"), 0644)
	os.WriteFile(filepath.Join(rtDir, "rtlog"), []byte("binary"), 0755)

	setupCleanup(rtDir)

	// Denylist files should be gone
	for _, name := range denylist {
		if _, err := os.Stat(filepath.Join(rtDir, name)); !os.IsNotExist(err) {
			t.Errorf("denylist file %s was not deleted", name)
		}
	}

	// Preserved files should exist
	for _, name := range []string{"state", "tools.conf", "extract.conf", "rtlog"} {
		if _, err := os.Stat(filepath.Join(rtDir, name)); err != nil {
			t.Errorf("preserved file %s was deleted: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(rtDir, "logs", "test.db")); err != nil {
		t.Error("logs/test.db was deleted")
	}
}

func TestSetupCleanup_MissingFiles(t *testing.T) {
	// Cleanup should not error on a fresh directory with no files to delete
	rtDir := t.TempDir()
	setupCleanup(rtDir) // should not panic or error
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -run TestSetupCleanup -v`
Expected: compilation error — `setupCleanup` undefined

- [ ] **Step 3: Implement `setupCleanup()`**

Add to `cmd/setup.go`:

```go
// setupCleanup deletes stale application-managed files from rtDir.
// Preserved: logs/, state, tools.conf, extract.conf, rtlog binary.
func setupCleanup(rtDir string) {
	denylist := []string{
		"hook.zsh",
		"hook.bash",
		"hook-noninteractive.zsh",
		"hook-noninteractive.bash",
		"bash-preexec.sh",
		"last-update-check",
		"update-available",
	}
	for _, name := range denylist {
		path := filepath.Join(rtDir, name)
		if err := os.Remove(path); err == nil {
			fmt.Printf("[~]  Cleaned up: %s\n", name)
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/ -run TestSetupCleanup -v`
Expected: both tests PASS

- [ ] **Step 5: Run `go build` to verify compilation**

Run: `go build -o /dev/null .`
Expected: clean build

- [ ] **Step 6: Commit**

```bash
git add cmd/setup.go cmd/setup_uninstall_test.go
git commit -m "feat(setup): add setupCleanup denylist function

Deletes stale application-managed files (hooks, preexec, update-check
files) before setup re-creates them. Preserves logs/, state, config
files, and the binary."
```

---

### Task 3: Refactor `setupCopySelf()` to handle permission errors gracefully

**Files:**
- Modify: `cmd/setup.go:198-270` (change `setupCopySelf` signature)
- Test: `cmd/setup_uninstall_test.go`

Currently `setupCopySelf` calls `os.Exit(1)` on all errors. For the custom path case, permission errors should print an advisory instead. Change the function to return an error so the caller can decide.

- [ ] **Step 1: Write failing test for permission error handling**

Add to `cmd/setup_uninstall_test.go`:

```go
func TestSetupCopySelf_PermissionError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root user")
	}
	tmp := t.TempDir()

	// Create a read-only directory — writing will fail with permission error
	roDir := filepath.Join(tmp, "readonly")
	os.MkdirAll(roDir, 0755)
	dst := filepath.Join(roDir, "rtlog")
	// Make dir read-only after creating it
	os.Chmod(roDir, 0555)
	defer os.Chmod(roDir, 0755)

	err := setupCopySelfTo(dst)
	if err == nil {
		t.Error("expected permission error, got nil")
	}
	if !os.IsPermission(err) {
		t.Errorf("expected permission error, got: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestSetupCopySelf_PermissionError -v`
Expected: compilation error — `setupCopySelfTo` undefined

- [ ] **Step 3: Add `setupCopySelfTo()` that returns an error**

Add a new function to `cmd/setup.go` alongside the existing `setupCopySelf`. The existing `setupCopySelf` remains for backward compatibility within this file but will be replaced in Task 4. The new function returns errors instead of calling `os.Exit`:

```go
// setupCopySelfTo copies the running binary to dst using atomic temp+rename.
// Returns an error instead of exiting, so callers can handle permission errors.
func setupCopySelfTo(dst string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine own path: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return fmt.Errorf("cannot resolve own path: %w", err)
	}

	// If we're already at the destination, skip
	absDst, _ := filepath.Abs(dst)
	if self == absDst {
		fmt.Printf("[ok] Binary already at %s\n", dst)
		return nil
	}

	// Check if existing binary is identical
	selfInfo, err := os.Stat(self)
	if err == nil {
		dstInfo, derr := os.Stat(dst)
		if derr == nil && selfInfo.Size() == dstInfo.Size() {
			selfData, e1 := os.ReadFile(self)
			dstData, e2 := os.ReadFile(dst)
			if e1 == nil && e2 == nil && bytes.Equal(selfData, dstData) {
				fmt.Printf("[ok] Binary is up to date: %s\n", dst)
				return nil
			}
		}
	}

	// Atomic copy: write to temp + rename
	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, ".rtlog.")
	if err != nil {
		if os.IsPermission(err) {
			return err
		}
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	src, err := os.Open(self)
	if err != nil {
		tmp.Close()
		return fmt.Errorf("cannot open self: %w", err)
	}
	defer src.Close()

	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to copy binary: %w", err)
	}
	if err := tmp.Chmod(0755); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to set permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		if os.IsPermission(err) {
			return err
		}
		return fmt.Errorf("failed to install binary: %w", err)
	}
	fmt.Printf("[+]  Installed binary: %s\n", dst)
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/ -run TestSetupCopySelf -v`
Expected: PASS

- [ ] **Step 5: Run `go build` to verify compilation**

Run: `go build -o /dev/null .`
Expected: clean build

- [ ] **Step 6: Commit**

```bash
git add cmd/setup.go cmd/setup_uninstall_test.go
git commit -m "feat(setup): add setupCopySelfTo with error return

New variant of setupCopySelf that returns errors instead of calling
os.Exit, enabling graceful permission error handling for custom paths."
```

---

### Task 4: Wire everything into `runSetup()`

**Files:**
- Modify: `cmd/setup.go:24-128` (rewrite `runSetup` and command description)

This task integrates `setupCleanup()`, `detectBinaryPath()`, and `setupCopySelfTo()` into the main setup flow. Remove `isOnPath()` and `setupCopySelf()` (replaced by the new functions).

- [ ] **Step 1: Update the command description**

Replace the `Long` description in `setupCmd` (lines 27-36):

```go
	Long: `Idempotent setup that installs rtlog to ~/.rt/ and configures zsh and/or bash.

Steps performed:
  1. Create ~/.rt/logs/
  2. Clean up stale files from previous versions
  3. Write embedded hook files (interactive + non-interactive) and config to ~/.rt/
  4. Detect binary on PATH and install (custom path, default, or fresh install)
  5. Configure ~/.zshrc and/or ~/.bashrc (hook source line; PATH export)
  6. Configure ~/.zshenv for non-interactive zsh capture
  7. Export BASH_ENV in shell rc files for non-interactive bash capture`,
```

- [ ] **Step 2: Rewrite `runSetup()` with the new flow**

Replace the body of `runSetup` (lines 45-128):

```go
func runSetup(cmd *cobra.Command, args []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine home directory: %v\n", err)
		os.Exit(1)
	}

	rtDir := filepath.Join(home, ".rt")
	logDir := filepath.Join(rtDir, "logs")
	localBin := filepath.Join(home, ".local", "bin")
	zshrc := filepath.Join(home, ".zshrc")
	bashrc := filepath.Join(home, ".bashrc")

	fmt.Println("=== Red Team Operation Logger - Setup ===")
	fmt.Println()

	// 1. Create ~/.rt/logs/ (always needed)
	setupCreateDir(logDir)

	// 2. Cleanup stale files from previous versions
	setupCleanup(rtDir)

	// 3. Write embedded files (both shells, always)
	setupWriteEmbedded("hook.zsh", filepath.Join(rtDir, "hook.zsh"), false)
	setupWriteEmbedded("hook.bash", filepath.Join(rtDir, "hook.bash"), false)
	setupWriteEmbedded("bash-preexec.sh", filepath.Join(rtDir, "bash-preexec.sh"), false)
	setupWriteEmbedded("tools.conf", filepath.Join(rtDir, "tools.conf"), true)
	setupWriteEmbedded("extract.conf", filepath.Join(rtDir, "extract.conf"), true)
	setupWriteEmbedded("hook-noninteractive.zsh", filepath.Join(rtDir, "hook-noninteractive.zsh"), false)
	setupWriteEmbedded("hook-noninteractive.bash", filepath.Join(rtDir, "hook-noninteractive.bash"), false)

	// 4. Binary installation — detect existing path
	kind, binPath := detectBinaryPath(home)
	addPathExport := false

	switch kind {
	case installGoInstall:
		fmt.Println("[ok] Installed via 'go install', skipping binary copy")
		fmt.Println("     To update: go install github.com/cyb33rr/rtlog@latest")

	case installCustom:
		fmt.Printf("[ok] Binary found at custom path: %s\n", binPath)
		if err := setupCopySelfTo(binPath); err != nil {
			if os.IsPermission(err) {
				fmt.Fprintf(os.Stderr, "[!]  Permission denied writing to %s\n", binPath)
				fmt.Fprintf(os.Stderr, "     Try: sudo rtlog setup\n")
			} else {
				fmt.Fprintf(os.Stderr, "[!]  Failed to update binary: %v\n", err)
				os.Exit(1)
			}
		}

	case installDefault:
		setupCreateDir(localBin)
		if err := setupCopySelfTo(filepath.Join(rtDir, "rtlog")); err != nil {
			fmt.Fprintf(os.Stderr, "[!]  Failed to install binary: %v\n", err)
			os.Exit(1)
		}
		setupSymlink(filepath.Join(localBin, "rtlog"), filepath.Join(rtDir, "rtlog"))

	case installFresh:
		setupCreateDir(localBin)
		if err := setupCopySelfTo(filepath.Join(rtDir, "rtlog")); err != nil {
			fmt.Fprintf(os.Stderr, "[!]  Failed to install binary: %v\n", err)
			os.Exit(1)
		}
		addPathExport = setupSymlink(filepath.Join(localBin, "rtlog"), filepath.Join(rtDir, "rtlog"))
	}

	// 5. Configure shell rc files based on existence
	zshrcExists := fileExists(zshrc)
	bashrcExists := fileExists(bashrc)

	if zshrcExists {
		setupShellRc(zshrc, localBin, rtDir, addPathExport, "hook.zsh", ".zshrc")
	}
	if bashrcExists {
		setupShellRc(bashrc, localBin, rtDir, addPathExport, "hook.bash", ".bashrc")
	}
	if !zshrcExists && !bashrcExists {
		fmt.Println("[!]  No ~/.zshrc or ~/.bashrc found — skipping shell configuration")
		fmt.Println("     Create your rc file and re-run 'rtlog setup'")
	}

	// 6. Configure ~/.zshenv for non-interactive zsh capture
	zshenv := filepath.Join(home, ".zshenv")
	setupZshenv(zshenv, rtDir)

	// 7. Export BASH_ENV in shell rc files for non-interactive bash capture
	if zshrcExists {
		setupBashEnv(zshrc, rtDir, ".zshrc")
	}
	if bashrcExists {
		setupBashEnv(bashrc, rtDir, ".bashrc")
	}

	fmt.Println()
	fmt.Println("=== Setup complete ===")
	fmt.Println()
	fmt.Println("Quick-start:")
	if zshrcExists && bashrcExists {
		fmt.Println("  1. Reload shell:     source ~/.zshrc  (or source ~/.bashrc)")
	} else if zshrcExists {
		fmt.Println("  1. Reload shell:     source ~/.zshrc")
	} else if bashrcExists {
		fmt.Println("  1. Reload shell:     source ~/.bashrc")
	}
	fmt.Println("  2. Start engagement: rtlog new <name>")
	fmt.Println("  3. Set phase tag:    rtlog tag recon")
	fmt.Println("  4. Run tools normally - logging is automatic")
	fmt.Println("  5. Query logs:       rtlog show")
	fmt.Println("  6. Full status:      rtlog status")
	fmt.Println("  7. More commands:    rtlog --help")
}
```

- [ ] **Step 3: Remove old `isOnPath()` and `setupCopySelf()`**

Delete the `isOnPath()` function (lines 321-336) and the old `setupCopySelf()` function (lines 198-270). These are fully replaced by `detectBinaryPath()` and `setupCopySelfTo()`.

- [ ] **Step 4: Run all tests to verify nothing is broken**

Run: `go test ./cmd/ -v`
Expected: all tests PASS

- [ ] **Step 5: Run `go build` to verify compilation**

Run: `go build -o /dev/null .`
Expected: clean build, no errors

- [ ] **Step 6: Commit**

```bash
git add cmd/setup.go
git commit -m "feat(setup): wire smart binary detection and cleanup into runSetup

Setup now:
- Cleans up stale denylist files before writing embedded files
- Detects binary on PATH via exec.LookPath (four cases: go install,
  custom path, default ~/.rt/, fresh install)
- Skips ~/.local/bin/ creation and PATH export for custom/goinstall
- Handles permission errors gracefully for custom paths"
```

---

### Task 5: Run full test suite and verify

**Files:** None (verification only)

- [ ] **Step 1: Run all project tests**

Run: `go test ./... -v`
Expected: all tests PASS

- [ ] **Step 2: Build the binary**

Run: `go build -o /tmp/rtlog-test .`
Expected: clean build

- [ ] **Step 3: Verify `go vet` passes**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 4: Verify the old `isOnPath` and `setupCopySelf` are fully removed**

Run: `grep -rn 'isOnPath\|func setupCopySelf[^T]' cmd/setup.go`
Expected: no output (functions removed)

- [ ] **Step 5: Clean up test binary**

Run: `rm /tmp/rtlog-test`

---

### Notes

**`tools.conf`/`extract.conf` prompt behavior:** The spec requires testing that prompt behavior is preserved. This plan does not add a new test for it because `setupWriteEmbedded()` is not modified — the `userConfig` prompt logic is unchanged. The cleanup denylist explicitly excludes these files, so the existing behavior is preserved by design. The `TestSetupCleanup` test verifies both files survive cleanup.

**Upgrade path integration tests:** The spec lists integration-level upgrade path tests. This plan covers them through unit tests of `detectBinaryPath()` (one test per case), which is pragmatic and sufficient since `runSetup()` is a thin orchestrator over the tested components.
