# Unify Setup, Uninstall, and Update Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Simplify setup/uninstall/update around the single assumption that Go is required and the binary lives in Go's bin directory. Update calls setup after `go install`, old install paths are migrated, dead code is removed.

**Architecture:** Extract setup's core logic into `setupCore(home string) error` callable by both `runSetup` and `runUpdate`. Simplify `setupShellRc` to only manage Go bin PATH exports (removing `~/.local/bin` logic). Add migration for old installs. Simplify uninstall by removing symlink handling and using `resolveGoBinDir` for binary advise.

**Tech Stack:** Go, standard library, Cobra CLI, `golang.org/x/term`

**Spec:** `docs/superpowers/specs/2026-03-20-unify-setup-uninstall-update-design.md`

---

### Task 1: Refactor setup helpers to return errors instead of `os.Exit`

**Files:**
- Modify: `cmd/setup.go:164-519` (setupCreateDir, setupWriteEmbedded, setupShellRc)

Currently `setupCreateDir`, `setupWriteEmbedded`, and `setupShellRc` call `os.Exit(1)` on failure. These must return errors so `setupCore` can propagate them to callers (including `runUpdate`).

- [ ] **Step 1: Change `setupCreateDir` to return error**

In `cmd/setup.go`, change:

```go
func setupCreateDir(dir string) {
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		fmt.Printf("[ok] Directory exists: %s\n", dir)
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to create %s: %v\n", dir, err)
		os.Exit(1)
	}
	fmt.Printf("[+]  Created directory: %s\n", dir)
}
```

To:

```go
func setupCreateDir(dir string) error {
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		fmt.Printf("[ok] Directory exists: %s\n", dir)
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", dir, err)
	}
	fmt.Printf("[+]  Created directory: %s\n", dir)
	return nil
}
```

- [ ] **Step 2: Change `setupWriteEmbedded` to return error**

Change signature from `func setupWriteEmbedded(name, dst string, userConfig bool)` to `func setupWriteEmbedded(name, dst string, userConfig bool) error`. Replace every `os.Exit(1)` with `return fmt.Errorf(...)`. Add `return nil` at the end. The user-prompt path (declining overwrite) returns `nil`.

```go
func setupWriteEmbedded(name, dst string, userConfig bool) error {
	data, err := embeddedFS.ReadFile(name)
	if err != nil {
		return fmt.Errorf("embedded file %s not found: %w", name, err)
	}

	existing, err := os.ReadFile(dst)
	if err == nil && bytes.Equal(existing, data) {
		fmt.Printf("[ok] %s is up to date\n", name)
		return nil
	}

	if userConfig && err == nil {
		fmt.Printf("[?]  %s has been modified. Overwrite with defaults? [y/N] ", name)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Printf("[ok] Keeping existing %s\n", name)
			return nil
		}
	}

	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, "."+name+".")
	if err != nil {
		return fmt.Errorf("failed to create temp file for %s: %w", name, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close %s: %w", name, err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return fmt.Errorf("failed to install %s: %w", name, err)
	}
	fmt.Printf("[+]  Installed %s -> %s\n", name, dst)
	return nil
}
```

- [ ] **Step 3: Change `setupShellRc` to return error**

Change signature to return `error`. Replace every `os.Exit(1)` with `return fmt.Errorf(...)`. Add `return nil` at the end.

The key changes in the function body:

```go
func setupShellRc(rcFile, localBin, rtDir string, addPathExport bool, goBinExportLine, hookFile, rcName string) error {
	// ... existing scanning logic stays the same ...

	// Atomic write section: replace os.Exit calls with return error
	dir := filepath.Dir(rcFile)
	tmp, err := os.CreateTemp(dir, "."+rcName+".")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.WriteString(newContent); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write %s: %w", rcName, err)
	}
	if info, err := os.Stat(rcFile); err == nil {
		tmp.Chmod(info.Mode())
	} else {
		tmp.Chmod(0644)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	if err := os.Rename(tmpName, rcFile); err != nil {
		return fmt.Errorf("failed to update %s: %w", rcName, err)
	}
	return nil
}
```

Also change the initial read error:
```go
if err != nil && !os.IsNotExist(err) {
	return fmt.Errorf("cannot read %s: %w", rcFile, err)
}
```

- [ ] **Step 4: Update `runSetup` to handle returned errors**

In `runSetup`, update every call to `setupCreateDir`, `setupWriteEmbedded`, and `setupShellRc` to check the returned error and call `os.Exit(1)` on failure. For example:

```go
if err := setupCreateDir(logDir); err != nil {
	fmt.Fprintf(os.Stderr, "[!]  %v\n", err)
	os.Exit(1)
}
```

Apply the same pattern to every `setupWriteEmbedded` and `setupShellRc` call in `runSetup`. Note: `runSetup` will be replaced by `setupCore` in Task 2, but it must compile and pass tests after this task.

- [ ] **Step 5: Run all tests to verify no regressions**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -v`
Expected: ALL PASS (existing tests call helpers directly and don't test os.Exit behavior)

- [ ] **Step 6: Commit**

```bash
git add cmd/setup.go
git commit -m "refactor(setup): return errors from helpers instead of os.Exit"
```

---

### Task 2: Extract `setupCore` and wire `runSetup`

**Files:**
- Modify: `cmd/setup.go:25-161` (setupCmd, runSetup, new setupCore)

- [ ] **Step 1: Update the setup command description**

Change the `Long` description in `setupCmd`:

```go
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure rtlog shell hooks and environment",
	Long: `Idempotent setup that configures zsh and/or bash for rtlog.

Requires Go toolchain (binary installed via 'go install').

Steps performed:
  1. Create ~/.rt/logs/
  2. Clean up stale files from previous versions
  3. Migrate old installs (remove ~/.rt/rtlog binary, ~/.local/bin symlink)
  4. Write embedded hook files (interactive + non-interactive) and config to ~/.rt/
  5. Resolve Go bin directory and ensure it is on PATH
  6. Configure ~/.zshrc and/or ~/.bashrc (hook source line; Go bin PATH export)
  7. Configure ~/.zshenv for non-interactive zsh capture
  8. Export BASH_ENV in shell rc files for non-interactive bash capture`,
	Args: cobra.NoArgs,
	Run:  runSetup,
}
```

- [ ] **Step 2: Create `setupCore` function**

Add after the `init()` function, before `runSetup`:

```go
// setupCore runs all setup steps and returns an error on failure.
// Called by both runSetup (standalone) and runUpdate (after go install).
func setupCore(home string) error {
	rtDir := filepath.Join(home, ".rt")
	logDir := filepath.Join(rtDir, "logs")
	zshrc := filepath.Join(home, ".zshrc")
	bashrc := filepath.Join(home, ".bashrc")

	// 1. Create directories
	if err := setupCreateDir(logDir); err != nil {
		return err
	}

	// 2. Cleanup stale files
	setupCleanup(rtDir)

	// 3. Write embedded files
	embeds := []struct {
		name       string
		dst        string
		userConfig bool
	}{
		{"hook.zsh", filepath.Join(rtDir, "hook.zsh"), false},
		{"hook.bash", filepath.Join(rtDir, "hook.bash"), false},
		{"bash-preexec.sh", filepath.Join(rtDir, "bash-preexec.sh"), false},
		{"tools.conf", filepath.Join(rtDir, "tools.conf"), true},
		{"extract.conf", filepath.Join(rtDir, "extract.conf"), true},
		{"hook-noninteractive.zsh", filepath.Join(rtDir, "hook-noninteractive.zsh"), false},
		{"hook-noninteractive.bash", filepath.Join(rtDir, "hook-noninteractive.bash"), false},
	}
	for _, f := range embeds {
		if err := setupWriteEmbedded(f.name, f.dst, f.userConfig); err != nil {
			return err
		}
	}

	// 4. Resolve Go bin dir
	_, goBinExportLine := resolveGoBinDir(home, os.Getenv("GOPATH"), os.Getenv("GOBIN"))

	// 5. Configure shell rc files
	zshrcExists := fileExists(zshrc)
	bashrcExists := fileExists(bashrc)

	if zshrcExists {
		if err := setupShellRc(zshrc, "", rtDir, false, goBinExportLine, "hook.zsh", ".zshrc"); err != nil {
			return err
		}
	}
	if bashrcExists {
		if err := setupShellRc(bashrc, "", rtDir, false, goBinExportLine, "hook.bash", ".bashrc"); err != nil {
			return err
		}
	}
	if !zshrcExists && !bashrcExists {
		fmt.Println("[!]  No ~/.zshrc or ~/.bashrc found — skipping shell configuration")
		fmt.Println("     Create your rc file and re-run 'rtlog setup'")
	}

	// 6. Configure .zshenv for non-interactive zsh capture
	zshenv := filepath.Join(home, ".zshenv")
	setupZshenv(zshenv, rtDir)

	// 7. BASH_ENV for non-interactive bash capture
	if zshrcExists {
		setupBashEnv(zshrc, rtDir, ".zshrc")
	}
	if bashrcExists {
		setupBashEnv(bashrc, rtDir, ".bashrc")
	}

	return nil
}
```

Note: `setupShellRc` still uses the old 7-param signature here (`localBin` passed as `""`, `addPathExport` as `false`). Task 3 will simplify this to a 4-param signature.

- [ ] **Step 3: Rewrite `runSetup` to call `setupCore`**

```go
func runSetup(cmd *cobra.Command, args []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine home directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Red Team Operation Logger - Setup ===")
	fmt.Println()

	if err := setupCore(home); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  %v\n", err)
		os.Exit(1)
	}

	zshrc := filepath.Join(home, ".zshrc")
	bashrc := filepath.Join(home, ".bashrc")
	zshrcExists := fileExists(zshrc)
	bashrcExists := fileExists(bashrc)

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

- [ ] **Step 4: Run all tests**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/setup.go
git commit -m "refactor(setup): extract setupCore returning errors for reuse by update"
```

---

### Task 3: Add migration and simplify `setupShellRc`

**Files:**
- Modify: `cmd/setup.go` (setupCleanup, new setupMigrateSymlink, setupShellRc, setupCore)
- Modify: `cmd/setup_uninstall_test.go` (update existing tests, add migration tests)

- [ ] **Step 1: Write failing test for `~/.rt/rtlog` cleanup**

Update `TestSetupCleanup` in `cmd/setup_uninstall_test.go`. The `rtlog` binary should now be in the denylist (deleted), not preserved:

```go
func TestSetupCleanup(t *testing.T) {
	rtDir := t.TempDir()

	// Create denylist files (should be deleted)
	denylist := []string{
		"hook.zsh", "hook.bash",
		"hook-noninteractive.zsh", "hook-noninteractive.bash",
		"bash-preexec.sh", "last-update-check", "update-available",
		"rtlog",
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

	setupCleanup(rtDir)

	// Denylist files should be gone
	for _, name := range denylist {
		if _, err := os.Stat(filepath.Join(rtDir, name)); !os.IsNotExist(err) {
			t.Errorf("denylist file %s was not deleted", name)
		}
	}

	// Preserved files should exist
	for _, name := range []string{"state", "tools.conf", "extract.conf"} {
		if _, err := os.Stat(filepath.Join(rtDir, name)); err != nil {
			t.Errorf("preserved file %s was deleted: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(rtDir, "logs", "test.db")); err != nil {
		t.Error("logs/test.db was deleted")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -run TestSetupCleanup -v`
Expected: FAIL — `rtlog` not deleted (still in preserved list)

- [ ] **Step 3: Add `rtlog` to the cleanup denylist**

In `cmd/setup.go`, add `"rtlog"` to the denylist in `setupCleanup`:

```go
func setupCleanup(rtDir string) {
	denylist := []string{
		"hook.zsh",
		"hook.bash",
		"hook-noninteractive.zsh",
		"hook-noninteractive.bash",
		"bash-preexec.sh",
		"last-update-check",
		"update-available",
		"rtlog",
	}
	for _, name := range denylist {
		path := filepath.Join(rtDir, name)
		if err := os.Remove(path); err == nil {
			fmt.Printf("[~]  Cleaned up: %s\n", name)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -run TestSetupCleanup -v`
Expected: PASS

- [ ] **Step 5: Write failing tests for symlink migration**

Add to `cmd/setup_uninstall_test.go`:

```go
func TestSetupMigrateSymlink(t *testing.T) {
	tmp := t.TempDir()
	localBin := filepath.Join(tmp, ".local", "bin")
	os.MkdirAll(localBin, 0755)
	rtBinary := filepath.Join(tmp, ".rt", "rtlog")

	// Create symlink pointing to our binary
	link := filepath.Join(localBin, "rtlog")
	os.Symlink(rtBinary, link)

	setupMigrateSymlink(link, rtBinary)

	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Error("symlink was not removed")
	}
}

func TestSetupMigrateSymlink_NonMatching(t *testing.T) {
	tmp := t.TempDir()
	localBin := filepath.Join(tmp, ".local", "bin")
	os.MkdirAll(localBin, 0755)

	// Create symlink pointing elsewhere
	link := filepath.Join(localBin, "rtlog")
	os.Symlink("/usr/local/bin/rtlog", link)

	setupMigrateSymlink(link, filepath.Join(tmp, ".rt", "rtlog"))

	// Should be left alone
	if _, err := os.Lstat(link); err != nil {
		t.Error("non-matching symlink was incorrectly removed")
	}
}

func TestSetupMigrateSymlink_RegularFile(t *testing.T) {
	tmp := t.TempDir()
	localBin := filepath.Join(tmp, ".local", "bin")
	os.MkdirAll(localBin, 0755)

	// Create regular file (not a symlink)
	file := filepath.Join(localBin, "rtlog")
	os.WriteFile(file, []byte("binary"), 0755)

	setupMigrateSymlink(file, filepath.Join(tmp, ".rt", "rtlog"))

	// Should be left alone
	if _, err := os.Stat(file); err != nil {
		t.Error("regular file was incorrectly removed")
	}
}

func TestSetupMigrateSymlink_NotExists(t *testing.T) {
	// Should not error when symlink doesn't exist
	setupMigrateSymlink("/nonexistent/path", "/also/nonexistent")
}
```

- [ ] **Step 6: Run tests to verify they fail**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -run TestSetupMigrateSymlink -v`
Expected: FAIL — `setupMigrateSymlink` undefined

- [ ] **Step 7: Implement `setupMigrateSymlink`**

Add to `cmd/setup.go` after `setupCleanup`:

```go
// setupMigrateSymlink removes a symlink at link if it points to expectedTarget.
// Non-matching symlinks and regular files are left alone.
func setupMigrateSymlink(link, expectedTarget string) {
	target, err := os.Readlink(link)
	if err != nil {
		return // not a symlink or doesn't exist
	}
	if target != expectedTarget {
		return // points elsewhere, leave it alone
	}
	if err := os.Remove(link); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to remove old symlink %s: %v\n", link, err)
		return
	}
	fmt.Printf("[~]  Removed old symlink: %s\n", link)
}
```

- [ ] **Step 8: Run migration tests**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -run TestSetupMigrateSymlink -v`
Expected: ALL PASS (4 sub-tests)

- [ ] **Step 9: Write failing test for `setupShellRc` migration of `~/.local/bin` PATH export**

Add to `cmd/setup_uninstall_test.go`:

```go
func TestSetupShellRcMigratesLocalBinExport(t *testing.T) {
	tmp := t.TempDir()
	bashrc := filepath.Join(tmp, ".bashrc")

	// Simulate old setup: has ~/.local/bin PATH export
	content := strings.Join([]string{
		"# my config",
		`export PATH="$HOME/.local/bin:$PATH"`,
		"# Red Team Operation Logger",
		"source $HOME/.rt/hook.bash",
	}, "\n")
	os.WriteFile(bashrc, []byte(content), 0644)

	setupShellRc(bashrc, `export PATH="$HOME/go/bin:$PATH"`, "hook.bash", ".bashrc")

	result, _ := os.ReadFile(bashrc)
	lines := string(result)

	// Old export should be removed
	if strings.Contains(lines, `$HOME/.local/bin`) {
		t.Error("old ~/.local/bin PATH export was not removed")
	}
	// Go bin export should be added
	if !strings.Contains(lines, `$HOME/go/bin`) {
		t.Error("Go bin PATH export not added")
	}
	// Hook source should still be present
	if !strings.Contains(lines, "source $HOME/.rt/hook.bash") {
		t.Error("hook source line was removed")
	}
}
```

- [ ] **Step 10: Run test to verify it fails**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -run TestSetupShellRcMigratesLocalBinExport -v`
Expected: FAIL — wrong number of arguments (old signature has 7 params, new has 4)

- [ ] **Step 11: Simplify `setupShellRc` signature and add migration logic**

Change `setupShellRc` in `cmd/setup.go`:

```go
// setupShellRc ensures Go bin PATH export and hook source lines are in the given rc file.
// Removes old ~/.local/bin PATH export if present (migration).
// hookFile is "hook.zsh" or "hook.bash". rcName is ".zshrc" or ".bashrc" (for messages).
func setupShellRc(rcFile, goBinExportLine, hookFile, rcName string) error {
	sourceLine := fmt.Sprintf("source %s/.rt/%s", "$HOME", hookFile)

	content, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot read %s: %w", rcFile, err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	hasSourceLine := false
	hasGoBinExport := false
	migrated := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Migration: remove old ~/.local/bin PATH export
		if !strings.HasPrefix(trimmed, "#") && trimmed == `export PATH="$HOME/.local/bin:$PATH"` {
			migrated = true
			continue
		}

		// Check for our source line
		if trimmed == sourceLine {
			hasSourceLine = true
		}

		// Check for existing Go bin PATH export
		if goBinExportLine != "" && !strings.HasPrefix(trimmed, "#") && trimmed == goBinExportLine {
			hasGoBinExport = true
		}

		newLines = append(newLines, line)
	}

	if migrated {
		newLines = collapseBlankLines(newLines)
		fmt.Printf("[~]  Removed old ~/.local/bin PATH export from %s\n", rcName)
	}

	// Append Go bin PATH export if needed
	if goBinExportLine != "" {
		if !hasGoBinExport {
			newLines = append(newLines, "", goBinExportLine)
			fmt.Printf("[+]  Added Go bin to PATH in %s\n", rcName)
		} else {
			fmt.Printf("[ok] Go bin already in PATH\n")
		}
	}

	// Append source line if missing
	if !hasSourceLine {
		newLines = append(newLines, "", "# Red Team Operation Logger", sourceLine)
		fmt.Printf("[+]  Added source line to %s\n", rcName)
	} else {
		fmt.Printf("[ok] %s already sourced in %s\n", hookFile, rcName)
	}

	// Atomic write
	newContent := strings.Join(newLines, "\n")
	if string(content) == newContent {
		return nil
	}

	dir := filepath.Dir(rcFile)
	tmp, err := os.CreateTemp(dir, "."+rcName+".")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.WriteString(newContent); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write %s: %w", rcName, err)
	}
	if info, err := os.Stat(rcFile); err == nil {
		tmp.Chmod(info.Mode())
	} else {
		tmp.Chmod(0644)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	if err := os.Rename(tmpName, rcFile); err != nil {
		return fmt.Errorf("failed to update %s: %w", rcName, err)
	}
	return nil
}
```

- [ ] **Step 12: Update `setupCore` to use new `setupShellRc` signature and add migration call**

In `setupCore`, after the cleanup step, add migration:

```go
// 3. Migrate old installs
setupMigrateSymlink(
	filepath.Join(home, ".local", "bin", "rtlog"),
	filepath.Join(rtDir, "rtlog"),
)
```

Update `setupShellRc` calls (remove `localBin`, `rtDir`, and `addPathExport` args):

```go
if zshrcExists {
	if err := setupShellRc(zshrc, goBinExportLine, "hook.zsh", ".zshrc"); err != nil {
		return err
	}
}
if bashrcExists {
	if err := setupShellRc(bashrc, goBinExportLine, "hook.bash", ".bashrc"); err != nil {
		return err
	}
}
```

- [ ] **Step 13: Update existing tests for new `setupShellRc` signature**

In `cmd/setup_uninstall_test.go`, update all `setupShellRc` call sites to use the new 4-parameter signature (remove `localBin`, `rtDir`, `addPathExport`):

`TestSetupShellRcBash`:
```go
setupShellRc(bashrc, "", "hook.bash", ".bashrc")
```

`TestSetupShellRcIdempotent`:
```go
setupShellRc(bashrc, "", "hook.bash", ".bashrc")
```

`TestSetupShellRcGoBinExport`:
```go
setupShellRc(bashrc, `export PATH="$HOME/go/bin:$PATH"`, "hook.bash", ".bashrc")
```

`TestSetupShellRcGoBinExportAlreadyPresent`:
```go
setupShellRc(bashrc, `export PATH="$HOME/go/bin:$PATH"`, "hook.bash", ".bashrc")
```

`TestSetupShellRcNoGoBinExport`:
```go
setupShellRc(bashrc, "", "hook.bash", ".bashrc")
```

- [ ] **Step 14: Run all tests**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -v`
Expected: ALL PASS

- [ ] **Step 15: Commit**

```bash
git add cmd/setup.go cmd/setup_uninstall_test.go
git commit -m "feat(setup): add migration for old installs and simplify setupShellRc"
```

---

### Task 4: Remove dead code and clean up tests

**Files:**
- Modify: `cmd/setup.go` (remove dead functions and types)
- Modify: `cmd/setup_uninstall_test.go` (remove dead tests)

- [ ] **Step 1: Remove dead code from `cmd/setup.go`**

Delete the following functions and types (in this order to avoid compilation issues):

1. The `installKind` type and its constants (lines 373-381):
```go
type installKind int

const (
	installFresh     installKind = iota
	installDefault
	installCustom
	installGoInstall
)
```

2. `detectBinaryPath` function (lines 383-414)

3. `setupCopySelfTo` function (lines 251-322)

4. `setupSymlink` function (lines 324-350)

5. `isGoInstalled` function (lines 625-636)

Also remove the `"io"` and `"os/exec"` imports — both become unused (`io` was only used by `setupCopySelfTo`, `os/exec` only by `detectBinaryPath`).

- [ ] **Step 2: Remove dead tests from `cmd/setup_uninstall_test.go`**

Delete:
1. `TestSetupSymlinkReturnValue` (lines 78-97)
2. `detectBinaryPathTestSetup` helper (lines 307-321)
3. `TestDetectBinaryPath_FreshInstall` (lines 323-336)
4. `TestDetectBinaryPath_DefaultPath` (lines 338-354)
5. `TestDetectBinaryPath_DefaultPathViaSymlink` (lines 356-375)
6. `TestDetectBinaryPath_CustomPath` (lines 377-393)
7. `TestDetectBinaryPath_GoInstall` (lines 395-413)
8. `TestSetupCopySelfTo_PermissionError` (lines 415-436)

- [ ] **Step 3: Verify compilation**

Run: `cd /home/cyb3r/RTLog && go build ./...`
Expected: Clean build

- [ ] **Step 4: Run all tests**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/setup.go cmd/setup_uninstall_test.go
git commit -m "refactor(setup): remove dead install cases and related code"
```

---

### Task 5: Wire `update.go` to call `setupCore`

**Files:**
- Modify: `cmd/update.go`
- Modify: `cmd/setup_uninstall_test.go` (add test)

- [ ] **Step 1: Verify `setupCore` integration compiles**

The key integration test for update→setup is that `runUpdate` calls `setupCore` and both compile. A full end-to-end test of update would require `embeddedFS` (injected from `main.go`) and a mock `go install`, which is out of scope for unit tests. The individual setup helpers are already well-tested in isolation.

The error-propagation behavior of `runUpdate` (skip setup when `go install` fails) is verified by code inspection: the `if err := goCmd.Run(); err != nil { return }` guard runs before `setupCore` is called.

Run: `cd /home/cyb3r/RTLog && go build ./...`
Expected: Clean build

- [ ] **Step 2: Update `runUpdate` to call `setupCore`**

In `cmd/update.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update rtlog to the latest version",
	Long: `Update rtlog via 'go install' and re-run setup to refresh hooks and config.

Requires Go toolchain.`,
	Args: cobra.NoArgs,
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	fmt.Println("Updating rtlog...")
	goCmd := exec.Command("go", "install", "github.com/cyb33rr/rtlog@latest")
	goCmd.Stdout = os.Stdout
	goCmd.Stderr = os.Stderr
	if err := goCmd.Run(); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	fmt.Println("Updated successfully.")
	fmt.Println()

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	fmt.Println("Running setup to refresh hooks and config...")
	if err := setupCore(home); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}
	fmt.Println("Setup complete.")

	return nil
}
```

- [ ] **Step 3: Verify compilation and run tests**

Run: `cd /home/cyb3r/RTLog && go build ./... && go test ./cmd/ -v`
Expected: Clean build, ALL PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/update.go
git commit -m "feat(update): call setupCore after go install to refresh hooks and config"
```

---

### Task 6: Simplify uninstall

**Files:**
- Modify: `cmd/uninstall.go`
- Modify: `cmd/setup_uninstall_test.go` (add test)

- [ ] **Step 1: Write failing test for uninstall advise using `resolveGoBinDir`**

Add to `cmd/setup_uninstall_test.go`:

```go
func TestUninstallOnOldInstall(t *testing.T) {
	// Uninstall should remove ~/.local/bin PATH export even without prior new setup
	tmp := t.TempDir()
	zshrc := filepath.Join(tmp, ".zshrc")

	content := strings.Join([]string{
		"# my config",
		`export PATH="$HOME/.local/bin:$PATH"`,
		"# Red Team Operation Logger",
		"source $HOME/.rt/hook.zsh",
		"alias ll='ls -la'",
	}, "\n")
	os.WriteFile(zshrc, []byte(content), 0644)

	uninstallCleanShellRc(zshrc, ".rt/hook.zsh", ".zshrc")

	result, _ := os.ReadFile(zshrc)
	lines := string(result)

	if strings.Contains(lines, `$HOME/.local/bin`) {
		t.Error("old ~/.local/bin PATH export was not removed by uninstall")
	}
	if !strings.Contains(lines, "alias ll='ls -la'") {
		t.Error("unrelated config was incorrectly removed")
	}
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -run TestUninstallOnOldInstall -v`
Expected: PASS (uninstall already removes `~/.local/bin` PATH export — line 138 of uninstall.go)

This test documents existing behavior. If it passes, the uninstall already handles this case correctly.

- [ ] **Step 3: Update the uninstall command description**

```go
var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove rtlog from the system",
	Long: `Remove rtlog installation artifacts:

  1. Remove hook and PATH export lines from ~/.zshrc and ~/.bashrc
  2. Remove non-interactive hook lines from ~/.zshenv
  3. Optionally delete ~/.rt/ (prompts unless -y)
  4. Advise how to remove the binary from Go's bin directory`,
	Args: cobra.NoArgs,
	Run:  runUninstall,
}
```

- [ ] **Step 4: Simplify `runUninstall` — remove symlink step, use `resolveGoBinDir` for advise**

```go
func runUninstall(cmd *cobra.Command, args []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine home directory: %v\n", err)
		os.Exit(1)
	}

	rtDir := filepath.Join(home, ".rt")
	zshrc := filepath.Join(home, ".zshrc")
	bashrc := filepath.Join(home, ".bashrc")

	fmt.Println("=== Red Team Operation Logger - Uninstaller ===")
	fmt.Println()

	// 1. Remove hook lines from shell rc files
	uninstallCleanShellRc(zshrc, ".rt/hook.zsh", ".zshrc")
	uninstallCleanShellRc(bashrc, ".rt/hook.bash", ".bashrc")

	// 2. Clean non-interactive hook lines
	zshenv := filepath.Join(home, ".zshenv")
	uninstallCleanNonInteractive(zshenv, ".zshenv")
	uninstallCleanNonInteractive(zshrc, ".zshrc")
	uninstallCleanNonInteractive(bashrc, ".bashrc")

	// 3. Remove ~/.rt/ (prompt first)
	uninstallRemoveDir(rtDir)

	// 4. Advise on binary removal
	dir, _ := resolveGoBinDir(home, os.Getenv("GOPATH"), os.Getenv("GOBIN"))
	binPath := filepath.Join(dir, "rtlog")
	fmt.Printf("[!]  Binary may be at %s\n", binPath)
	fmt.Println("     Remove it with: rm", binPath)

	fmt.Println()
	fmt.Println("=== Uninstall complete ===")
	fmt.Println()
	fmt.Println("Open a new shell to apply changes.")
}
```

- [ ] **Step 5: Delete dead functions from `cmd/uninstall.go`**

The `runUninstall` rewrite in Step 4 already excludes the dead variables and call sites. Now delete the function definitions themselves:
1. `uninstallRemoveSymlink` function (lines 79-102)
2. `uninstallAdviseGoInstall` function (lines 274-302)

The `"golang.org/x/term"` and `"bufio"` imports are still needed by `uninstallRemoveDir`.

- [ ] **Step 6: Verify compilation and run full test suite**

Run: `cd /home/cyb3r/RTLog && go build ./... && go test ./... -v`
Expected: Clean build, ALL PASS

- [ ] **Step 7: Commit**

```bash
git add cmd/uninstall.go cmd/setup_uninstall_test.go
git commit -m "refactor(uninstall): remove symlink step, use resolveGoBinDir for advise"
```
