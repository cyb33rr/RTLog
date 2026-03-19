# Old Artifact Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure `rtlog setup` and `rtlog uninstall` correctly clean all artifacts from every past version of RTLog, and fix temp file leaks in shell hooks.

**Architecture:** Five targeted fixes to existing setup/uninstall code and shell hooks. No new files or abstractions — each fix modifies existing functions in place. TDD: tests first, then implementation for Go changes. Shell hook changes are tested manually.

**Tech Stack:** Go (cmd package), zsh/bash shell scripts, Go testing

**Spec:** `docs/superpowers/specs/2026-03-20-old-artifact-cleanup-design.md`

---

### Task 1: Add `uninstall.sh` to cleanup denylist

**Files:**
- Modify: `cmd/setup_uninstall_test.go:239-278` (TestSetupCleanup)
- Modify: `cmd/setup.go:180-197` (setupCleanup)

- [ ] **Step 1: Update test to expect `uninstall.sh` in denylist**

In `TestSetupCleanup`, add `"uninstall.sh"` to both the denylist creation loop and the assertion loop:

```go
// In TestSetupCleanup, change the denylist slice to:
denylist := []string{
    "hook.zsh", "hook.bash",
    "hook-noninteractive.zsh", "hook-noninteractive.bash",
    "bash-preexec.sh", "last-update-check", "update-available",
    "rtlog", "uninstall.sh",
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestSetupCleanup -v`
Expected: FAIL — `uninstall.sh` not deleted

- [ ] **Step 3: Add `"uninstall.sh"` to denylist in `setupCleanup()`**

In `cmd/setup.go`, add to the denylist array at line 189:

```go
denylist := []string{
    "hook.zsh",
    "hook.bash",
    "hook-noninteractive.zsh",
    "hook-noninteractive.bash",
    "bash-preexec.sh",
    "last-update-check",
    "update-available",
    "rtlog",
    "uninstall.sh",
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/ -run TestSetupCleanup -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/setup.go cmd/setup_uninstall_test.go
git commit -m "feat(setup): add uninstall.sh to cleanup denylist"
```

---

### Task 2: Remove repo-based source lines in setup and uninstall

**Files:**
- Modify: `cmd/setup_uninstall_test.go` (add new tests)
- Modify: `cmd/setup.go:290-378` (setupShellRc)
- Modify: `cmd/uninstall.go:77-160` (uninstallCleanShellRc)

- [ ] **Step 1: Write test for setup migrating repo-based source lines**

Add to `cmd/setup_uninstall_test.go`:

```go
func TestSetupShellRcMigratesRepoSourceLines(t *testing.T) {
	tmp := t.TempDir()
	zshrc := filepath.Join(tmp, ".zshrc")

	content := strings.Join([]string{
		"# my config",
		"source ~/code/rtlog/hook.zsh",
		"source /opt/python-hook/hook.zsh",
		"source $HOME/.rt/hook.zsh",
		"alias ll='ls -la'",
	}, "\n")
	os.WriteFile(zshrc, []byte(content), 0644)

	setupShellRc(zshrc, "", "hook.zsh", ".zshrc")

	result, _ := os.ReadFile(zshrc)
	lines := string(result)

	// Old repo-based lines should be removed
	// Note: "rtlog/hook.zsh" is distinct from ".rt/hook.zsh" so no guard needed
	if strings.Contains(lines, "rtlog/hook.zsh") {
		t.Error("repo-based rtlog source line was not removed")
	}
	if strings.Contains(lines, "python-hook/hook.zsh") {
		t.Error("python-hook source line was not removed")
	}
	// Canonical source line should survive
	if !strings.Contains(lines, "source $HOME/.rt/hook.zsh") {
		t.Error("canonical .rt/hook.zsh source line was removed")
	}
	if !strings.Contains(lines, "alias ll='ls -la'") {
		t.Error("unrelated config was removed")
	}
}
```

- [ ] **Step 2: Write test for uninstall cleaning repo-based source lines**

Add to `cmd/setup_uninstall_test.go`:

```go
func TestUninstallCleansRepoSourceLines(t *testing.T) {
	tmp := t.TempDir()
	zshrc := filepath.Join(tmp, ".zshrc")

	content := strings.Join([]string{
		"# my config",
		"source ~/code/rtlog/hook.zsh",
		"source /opt/python-hook/hook.zsh",
		"# Red Team Operation Logger",
		"source $HOME/.rt/hook.zsh",
		"alias ll='ls -la'",
	}, "\n")
	os.WriteFile(zshrc, []byte(content), 0644)

	uninstallCleanShellRc(zshrc, ".rt/hook.zsh", ".zshrc")

	result, _ := os.ReadFile(zshrc)
	lines := string(result)

	if strings.Contains(lines, "rtlog/hook.zsh") {
		t.Error("repo-based rtlog source line was not removed")
	}
	if strings.Contains(lines, "python-hook/hook.zsh") {
		t.Error("python-hook source line was not removed")
	}
	if !strings.Contains(lines, "alias ll='ls -la'") {
		t.Error("unrelated config was removed")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./cmd/ -run "TestSetupShellRcMigratesRepoSourceLines|TestUninstallCleansRepoSourceLines" -v`
Expected: FAIL — repo-based lines not removed

- [ ] **Step 4: Add repo-based line removal to `setupShellRc()`**

In `cmd/setup.go`, inside the `for _, line := range lines` loop (after the `~/.local/bin` migration block at line 311), add:

```go
		// Migration: remove old repo-based or python-hook source lines
		if strings.Contains(trimmed, "source") &&
			(strings.Contains(trimmed, "/rtlog/hook.zsh") || strings.Contains(trimmed, "/python-hook/hook.zsh")) &&
			!strings.Contains(trimmed, ".rt/hook.") {
			migrated = true
			continue
		}
```

- [ ] **Step 5: Add repo-based line removal to `uninstallCleanShellRc()`**

In `cmd/uninstall.go`, inside the `for _, line := range lines` loop (after the `hookPattern` removal at line 105), add:

```go
		// Remove old repo-based or python-hook source lines
		if strings.Contains(trimmed, "source") &&
			(strings.Contains(trimmed, "/rtlog/hook.zsh") || strings.Contains(trimmed, "/python-hook/hook.zsh")) &&
			!strings.Contains(trimmed, ".rt/hook.") {
			removed = true
			continue
		}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./cmd/ -run "TestSetupShellRcMigratesRepoSourceLines|TestUninstallCleansRepoSourceLines" -v`
Expected: PASS

- [ ] **Step 7: Run all existing tests to verify no regressions**

Run: `go test ./cmd/ -v`
Expected: All PASS

- [ ] **Step 8: Commit**

```bash
git add cmd/setup.go cmd/uninstall.go cmd/setup_uninstall_test.go
git commit -m "feat(setup): remove old repo-based and python-hook source lines"
```

---

### Task 3: Tag Go bin PATH export for safe identification

This is the most complex task. It changes how setup writes the export line (tagged) and how uninstall removes it (match by tag). It also migrates existing untagged lines.

**Files:**
- Modify: `cmd/setup_uninstall_test.go` (update existing tests + add new)
- Modify: `cmd/setup.go:288-379` (setupShellRc) and `cmd/setup.go:485-504` (resolveGoBinDir)
- Modify: `cmd/uninstall.go:35-72` (runUninstall) and `cmd/uninstall.go:77-160` (uninstallCleanShellRc)

- [ ] **Step 1: Add `rtlogTag` constant and update `resolveGoBinDir` to return tagged line**

In `cmd/setup.go`, add a constant and modify `resolveGoBinDir`:

```go
// rtlogTag is appended to lines written by setup for safe identification during uninstall.
const rtlogTag = "  # added by rtlog"
```

Then change `resolveGoBinDir` line 502 from:
```go
exportLine = fmt.Sprintf(`export PATH="%s:$PATH"`, pathStr)
```
to:
```go
exportLine = fmt.Sprintf(`export PATH="%s:$PATH"%s`, pathStr, rtlogTag)
```

- [ ] **Step 2: Update `setupShellRc()` to detect both tagged and untagged lines**

In `setupShellRc()`, change the "already present" check (line 319) to match both forms. Replace:

```go
		// Check for existing Go bin PATH export
		if goBinExportLine != "" && !strings.HasPrefix(trimmed, "#") && trimmed == goBinExportLine {
			hasGoBinExport = true
		}
```

with:

```go
		// Check for existing Go bin PATH export (tagged or untagged)
		if goBinExportLine != "" && !strings.HasPrefix(trimmed, "#") {
			untagged := strings.TrimSuffix(goBinExportLine, rtlogTag)
			if trimmed == goBinExportLine {
				hasGoBinExport = true
			} else if trimmed == untagged {
				// Migration: replace untagged with tagged version
				hasGoBinExport = true
				migrated = true
				newLines = append(newLines, goBinExportLine)
				continue
			}
		}
```

- [ ] **Step 3: Update `uninstallCleanShellRc()` signature and matching**

Change `uninstallCleanShellRc` signature from:
```go
func uninstallCleanShellRc(rcFile, hookPattern, rcName string) {
```
to:
```go
func uninstallCleanShellRc(rcFile, hookPattern, rcName, goBinExportLine string) {
```

Replace the Go bin PATH export removal block (lines 113-117):
```go
		// Remove Go bin PATH export added by setup (default path)
		if trimmed == `export PATH="$HOME/go/bin:$PATH"` {
			removed = true
			continue
		}
```

with:
```go
		// Remove Go bin PATH export: tagged lines (any path) or untagged default (backward compat)
		if strings.Contains(trimmed, "export PATH=") && strings.HasSuffix(trimmed, rtlogTag) {
			removed = true
			continue
		}
		if trimmed == `export PATH="$HOME/go/bin:$PATH"` {
			removed = true
			continue
		}
```

- [ ] **Step 4: Update callers to pass `goBinExportLine`**

In `runUninstall()` (`cmd/uninstall.go`), change:
```go
	dir, _ := resolveGoBinDir(home, os.Getenv("GOPATH"), os.Getenv("GOBIN"))
```
to:
```go
	dir, goBinExportLine := resolveGoBinDir(home, os.Getenv("GOPATH"), os.Getenv("GOBIN"))
```

And update the calls:
```go
	uninstallCleanShellRc(zshrc, ".rt/hook.zsh", ".zshrc", goBinExportLine)
	uninstallCleanShellRc(bashrc, ".rt/hook.bash", ".bashrc", goBinExportLine)
```

Move the `resolveGoBinDir` call before the `uninstallCleanShellRc` calls (before current line 50).

- [ ] **Step 5: Update `TestResolveGoBinDir` expected values to include tag**

In `cmd/setup_uninstall_test.go`, update all `wantExport` values in `TestResolveGoBinDir` to include the tag. For each test case, append `"  # added by rtlog"`:

```go
{
    name:       "default (no GOBIN, no GOPATH)",
    wantExport: `export PATH="$HOME/go/bin:$PATH"  # added by rtlog`,
},
{
    name:       "custom GOPATH under home",
    wantExport: `export PATH="$HOME/mygo/bin:$PATH"  # added by rtlog`,
},
{
    name:       "custom GOPATH outside home",
    wantExport: `export PATH="/opt/gowork/bin:$PATH"  # added by rtlog`,
},
{
    name:       "GOBIN set under home",
    wantExport: `export PATH="$HOME/.gobin:$PATH"  # added by rtlog`,
},
{
    name:       "GOBIN set outside home",
    wantExport: `export PATH="/usr/local/gobin:$PATH"  # added by rtlog`,
},
{
    name:       "GOBIN takes priority over GOPATH",
    wantExport: `export PATH="$HOME/.gobin:$PATH"  # added by rtlog`,
},
```

- [ ] **Step 6: Update `TestSetupShellRcGoBinExport` to expect tagged line**

Change the assertion:
```go
if !strings.Contains(lines, `export PATH="$HOME/go/bin:$PATH"  # added by rtlog`) {
    t.Error("Go bin PATH export not added")
}
```

- [ ] **Step 7: Update `TestSetupShellRcGoBinExportAlreadyPresent` for tagged line**

The test file has the export line already present. Update both the initial content and the `setupShellRc` call to use the tagged version:

```go
func TestSetupShellRcGoBinExportAlreadyPresent(t *testing.T) {
	tmp := t.TempDir()
	bashrc := filepath.Join(tmp, ".bashrc")
	initial := strings.Join([]string{
		"# my config",
		`export PATH="$HOME/go/bin:$PATH"  # added by rtlog`,
		"",
		"# Red Team Operation Logger",
		"source $HOME/.rt/hook.bash",
	}, "\n")
	os.WriteFile(bashrc, []byte(initial), 0644)

	setupShellRc(bashrc, `export PATH="$HOME/go/bin:$PATH"  # added by rtlog`, "hook.bash", ".bashrc")

	result, _ := os.ReadFile(bashrc)
	if string(result) != initial {
		t.Errorf("setupShellRc modified file when Go bin export already present:\n%s", result)
	}
}
```

- [ ] **Step 8: Update `TestUninstallCleansCustomGoBinExport` for tagged behavior**

The test should now verify that a *tagged* custom export IS removed, while an *untagged* custom export is NOT:

```go
func TestUninstallCleansCustomGoBinExport(t *testing.T) {
	tmp := t.TempDir()
	zshrc := filepath.Join(tmp, ".zshrc")

	content := strings.Join([]string{
		"# my config",
		`export PATH="/opt/gowork/bin:$PATH"  # added by rtlog`,
		`export PATH="/usr/custom/bin:$PATH"`,
		"# Red Team Operation Logger",
		"source $HOME/.rt/hook.zsh",
		"alias ls='ls -la'",
	}, "\n")
	os.WriteFile(zshrc, []byte(content), 0644)

	uninstallCleanShellRc(zshrc, ".rt/hook.zsh", ".zshrc", `export PATH="/opt/gowork/bin:$PATH"  # added by rtlog`)

	result, _ := os.ReadFile(zshrc)
	lines := string(result)

	// Tagged custom export should be removed
	if strings.Contains(lines, `/opt/gowork/bin`) {
		t.Error("tagged custom PATH export was not removed")
	}
	// Untagged custom export should survive
	if !strings.Contains(lines, `/usr/custom/bin`) {
		t.Error("untagged custom export was incorrectly removed")
	}
	if !strings.Contains(lines, "alias ls='ls -la'") {
		t.Error("alias was incorrectly removed")
	}
}
```

- [ ] **Step 9: Add test for setup migrating untagged to tagged export**

```go
func TestSetupShellRcMigratesUntaggedExport(t *testing.T) {
	tmp := t.TempDir()
	bashrc := filepath.Join(tmp, ".bashrc")

	content := strings.Join([]string{
		"# my config",
		`export PATH="$HOME/go/bin:$PATH"`,
		"",
		"# Red Team Operation Logger",
		"source $HOME/.rt/hook.bash",
	}, "\n")
	os.WriteFile(bashrc, []byte(content), 0644)

	setupShellRc(bashrc, `export PATH="$HOME/go/bin:$PATH"  # added by rtlog`, "hook.bash", ".bashrc")

	result, _ := os.ReadFile(bashrc)
	lines := string(result)

	// Should have tagged version
	if !strings.Contains(lines, `# added by rtlog`) {
		t.Error("untagged export was not migrated to tagged version")
	}
	// Should not have duplicate
	if strings.Count(lines, `go/bin`) != 1 {
		t.Error("duplicate Go bin export found after migration")
	}
}
```

- [ ] **Step 10: Update remaining test callers of `uninstallCleanShellRc`**

All existing calls to `uninstallCleanShellRc` in tests need the 4th argument. Add `""` for tests that don't care about Go bin export:

- `TestUninstallNarrowHookMatching`: change to `uninstallCleanShellRc(zshrc, ".rt/hook.zsh", ".zshrc", "")`
- `TestUninstallBlankLineCollapsing`: change to `uninstallCleanShellRc(zshrc, ".rt/hook.zsh", ".zshrc", "")`
- `TestUninstallCleanBashrc`: change to `uninstallCleanShellRc(bashrc, ".rt/hook.bash", ".bashrc", "")`
- `TestUninstallBashrcDoesNotRemoveZshHook`: change to `uninstallCleanShellRc(bashrc, ".rt/hook.bash", ".bashrc", "")`
- `TestUninstallCleansGoBinExport`: change to `uninstallCleanShellRc(zshrc, ".rt/hook.zsh", ".zshrc", "")`
- `TestUninstallOnOldInstall`: change to `uninstallCleanShellRc(zshrc, ".rt/hook.zsh", ".zshrc", "")`

- [ ] **Step 11: Run all tests**

Run: `go test ./cmd/ -v`
Expected: All PASS

- [ ] **Step 12: Commit**

```bash
git add cmd/setup.go cmd/uninstall.go cmd/setup_uninstall_test.go
git commit -m "feat(setup): tag Go bin PATH export for safe uninstall identification"
```

---

### Task 4: Clean orphan temp files during setup

**Files:**
- Modify: `cmd/setup_uninstall_test.go` (add new test)
- Modify: `cmd/setup.go:178-197` (setupCleanup)

- [ ] **Step 1: Write test for temp file cleanup**

Add to `cmd/setup_uninstall_test.go`:

```go
func TestSetupCleanupTmpFiles(t *testing.T) {
	rtDir := t.TempDir()

	// Create orphan temp files in /tmp
	tmpFiles := []string{}
	for _, pattern := range []string{"/tmp/.rtlog_out.", "/tmp/.rtlog_ni_out."} {
		f, err := os.CreateTemp("/tmp", filepath.Base(pattern))
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		f.Close()
		tmpFiles = append(tmpFiles, f.Name())
	}
	defer func() {
		for _, f := range tmpFiles {
			os.Remove(f)
		}
	}()

	setupCleanup(rtDir)

	for _, f := range tmpFiles {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("orphan temp file was not cleaned: %s", f)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestSetupCleanupTmpFiles -v`
Expected: FAIL — temp files not deleted

- [ ] **Step 3: Add temp file cleanup to `setupCleanup()`**

In `cmd/setup.go`, after the denylist loop (after line 196), add:

```go
	// Clean orphan temp files from shell hooks
	for _, pattern := range []string{"/tmp/.rtlog_out.*", "/tmp/.rtlog_ni_out.*"} {
		matches, _ := filepath.Glob(pattern)
		for _, m := range matches {
			os.Remove(m) // errors silently ignored (e.g. EPERM on shared systems)
		}
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/ -run TestSetupCleanupTmpFiles -v`
Expected: PASS

- [ ] **Step 5: Run all tests**

Run: `go test ./cmd/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/setup.go cmd/setup_uninstall_test.go
git commit -m "feat(setup): clean orphan temp files from /tmp during setup"
```

---

### Task 5: On-demand temp file creation in shell hooks

**Files:**
- Modify: `hook.zsh:58-59` and `hook.zsh:138-145` and `hook.zsh:174-192`
- Modify: `hook.bash:61-62` and `hook.bash:143-150` and `hook.bash:179-197`
- Modify: `hook-noninteractive.bash:70-71` and `hook-noninteractive.bash:105-111` and `hook-noninteractive.bash:114-147`

- [ ] **Step 1: Update `hook.zsh` — lazy temp file creation**

Replace line 59:
```bash
_rtlog_tmpfile=$(mktemp /tmp/.rtlog_out.XXXXXXXX)
```
with:
```bash
_rtlog_tmpfile=""
```

Replace the output capture block in preexec (lines 139-145):
```bash
    if [[ "$RTLOG_CAPTURE" == "1" ]]; then
        : > "$_rtlog_tmpfile"
        exec {_rtlog_fd_out}>&1 {_rtlog_fd_err}>&2
        exec > >(tee -- "$_rtlog_tmpfile") 2>&1
        _rtlog_capturing=1
        (( RTLOG_DEBUG )) && echo "[rtlog:preexec] capturing output" >&2
    fi
```
with:
```bash
    if [[ "$RTLOG_CAPTURE" == "1" ]]; then
        _rtlog_tmpfile=$(mktemp /tmp/.rtlog_out.XXXXXXXX)
        exec {_rtlog_fd_out}>&1 {_rtlog_fd_err}>&2
        exec > >(tee -- "$_rtlog_tmpfile") 2>&1
        _rtlog_capturing=1
        (( RTLOG_DEBUG )) && echo "[rtlog:preexec] capturing output" >&2
    fi
```

Replace the cleanup in precmd (lines 187-192):
```bash
    command rm -f "$_rtlog_tmpfile" 2>/dev/null

    # Reset
    _rtlog_pending_tool=""
    _rtlog_pending_cmd=""
    _rtlog_pending_start=""
```
with:
```bash
    [[ -n "$_rtlog_tmpfile" ]] && command rm -f "$_rtlog_tmpfile" 2>/dev/null
    _rtlog_tmpfile=""

    # Reset
    _rtlog_pending_tool=""
    _rtlog_pending_cmd=""
    _rtlog_pending_start=""
```

- [ ] **Step 2: Update `hook.bash` — lazy temp file creation**

Replace line 62:
```bash
_rtlog_tmpfile=$(mktemp /tmp/.rtlog_out.XXXXXXXX)
```
with:
```bash
_rtlog_tmpfile=""
```

Replace the output capture block in preexec (lines 144-150):
```bash
    if [[ "$RTLOG_CAPTURE" == "1" ]]; then
        : > "$_rtlog_tmpfile"
        exec {_rtlog_fd_out}>&1 {_rtlog_fd_err}>&2
        exec > >(tee -- "$_rtlog_tmpfile") 2>&1
        _rtlog_capturing=1
        (( RTLOG_DEBUG )) && echo "[rtlog:preexec] capturing output" >&2
    fi
```
with:
```bash
    if [[ "$RTLOG_CAPTURE" == "1" ]]; then
        _rtlog_tmpfile=$(mktemp /tmp/.rtlog_out.XXXXXXXX)
        exec {_rtlog_fd_out}>&1 {_rtlog_fd_err}>&2
        exec > >(tee -- "$_rtlog_tmpfile") 2>&1
        _rtlog_capturing=1
        (( RTLOG_DEBUG )) && echo "[rtlog:preexec] capturing output" >&2
    fi
```

Replace the cleanup in precmd (lines 192-197):
```bash
    command rm -f "$_rtlog_tmpfile" 2>/dev/null

    # Reset
    _rtlog_pending_tool=""
    _rtlog_pending_cmd=""
    _rtlog_pending_start=""
```
with:
```bash
    [[ -n "$_rtlog_tmpfile" ]] && command rm -f "$_rtlog_tmpfile" 2>/dev/null
    _rtlog_tmpfile=""

    # Reset
    _rtlog_pending_tool=""
    _rtlog_pending_cmd=""
    _rtlog_pending_start=""
```

- [ ] **Step 3: Update `hook-noninteractive.bash` — lazy temp file creation**

Replace line 71:
```bash
_rtlog_ni_outfile=$(mktemp /tmp/.rtlog_ni_out.XXXXXXXX)
```
with:
```bash
_rtlog_ni_outfile=""
```

Replace the output capture block in the DEBUG handler (lines 106-111):
```bash
    if [[ "$_rtlog_ni_capture" == "1" ]]; then
        : > "$_rtlog_ni_outfile"
        exec {_rtlog_ni_fd_out}>&1 {_rtlog_ni_fd_err}>&2
        exec > >(tee -- "$_rtlog_ni_outfile") 2>&1
        _rtlog_ni_capturing=1
    fi
```
with:
```bash
    if [[ "$_rtlog_ni_capture" == "1" ]]; then
        _rtlog_ni_outfile=$(mktemp /tmp/.rtlog_ni_out.XXXXXXXX)
        exec {_rtlog_ni_fd_out}>&1 {_rtlog_ni_fd_err}>&2
        exec > >(tee -- "$_rtlog_ni_outfile") 2>&1
        _rtlog_ni_capturing=1
    fi
```

In the EXIT handler, add unconditional cleanup at the top (before line 126). Replace:
```bash
    [[ -n "$_rtlog_ni_pending_tool" ]] || return
```
with:
```bash
    if [[ -z "$_rtlog_ni_pending_tool" ]]; then
        [[ -n "$_rtlog_ni_outfile" ]] && command rm -f "$_rtlog_ni_outfile" 2>/dev/null
        return
    fi
```

And update the existing cleanup at line 146 to reset the var:
```bash
    [[ -n "$_rtlog_ni_outfile" ]] && command rm -f "$_rtlog_ni_outfile" 2>/dev/null
    _rtlog_ni_outfile=""
```

- [ ] **Step 4: Run `go test ./...` to verify no build breakage**

Run: `go test ./... -v`
Expected: All PASS (hook files are embedded, not compiled — this verifies Go code still builds)

- [ ] **Step 5: Commit**

```bash
git add hook.zsh hook.bash hook-noninteractive.bash
git commit -m "fix(hooks): create temp files on-demand to prevent leaks"
```

---

### Task 6: Final verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: All PASS

- [ ] **Step 2: Build and smoke test**

```bash
go build -o /tmp/rtlog-test .
```
Expected: Build succeeds

- [ ] **Step 3: Commit plan document**

```bash
git add docs/superpowers/plans/2026-03-20-old-artifact-cleanup.md
git commit -m "docs: add implementation plan for old artifact cleanup"
```
