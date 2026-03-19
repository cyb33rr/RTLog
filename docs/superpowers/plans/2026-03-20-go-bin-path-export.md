# Go Bin PATH Export Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure `rtlog setup` adds Go's bin directory to PATH in shell rc files when the binary was installed via `go install` and the directory isn't already exported.

**Architecture:** Add a `resolveGoBinDir` helper that computes the Go bin directory and a portable export line. Wire it into the `installGoInstall` case in `runSetup`, passing the export line to `setupShellRc` via a new parameter. Update `uninstallCleanShellRc` to also remove Go bin PATH exports.

**Tech Stack:** Go, standard library only

**Spec:** `docs/superpowers/specs/2026-03-20-go-bin-path-export-design.md`

---

### Task 1: Add `resolveGoBinDir` helper with tests

**Files:**
- Modify: `cmd/setup.go` (add helper after `isGoInstalled` at line 618)
- Modify: `cmd/setup_uninstall_test.go` (add tests at end)

- [ ] **Step 1: Write failing tests for `resolveGoBinDir`**

Add to `cmd/setup_uninstall_test.go`:

```go
func TestResolveGoBinDir(t *testing.T) {
	home := "/home/testuser"

	tests := []struct {
		name       string
		gobin      string
		gopath     string
		wantDir    string
		wantExport string
	}{
		{
			name:       "default (no GOBIN, no GOPATH)",
			gobin:      "",
			gopath:     "",
			wantDir:    filepath.Join(home, "go", "bin"),
			wantExport: `export PATH="$HOME/go/bin:$PATH"`,
		},
		{
			name:       "custom GOPATH under home",
			gobin:      "",
			gopath:     filepath.Join(home, "mygo"),
			wantDir:    filepath.Join(home, "mygo", "bin"),
			wantExport: `export PATH="$HOME/mygo/bin:$PATH"`,
		},
		{
			name:       "custom GOPATH outside home",
			gobin:      "",
			gopath:     "/opt/gowork",
			wantDir:    "/opt/gowork/bin",
			wantExport: `export PATH="/opt/gowork/bin:$PATH"`,
		},
		{
			name:       "GOBIN set under home",
			gobin:      filepath.Join(home, ".gobin"),
			gopath:     "",
			wantDir:    filepath.Join(home, ".gobin"),
			wantExport: `export PATH="$HOME/.gobin:$PATH"`,
		},
		{
			name:       "GOBIN set outside home",
			gobin:      "/usr/local/gobin",
			gopath:     "",
			wantDir:    "/usr/local/gobin",
			wantExport: `export PATH="/usr/local/gobin:$PATH"`,
		},
		{
			name:       "GOBIN takes priority over GOPATH",
			gobin:      filepath.Join(home, ".gobin"),
			gopath:     filepath.Join(home, "mygo"),
			wantDir:    filepath.Join(home, ".gobin"),
			wantExport: `export PATH="$HOME/.gobin:$PATH"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDir, gotExport := resolveGoBinDir(home, tt.gobin, tt.gopath)
			if gotDir != tt.wantDir {
				t.Errorf("dir = %q, want %q", gotDir, tt.wantDir)
			}
			if gotExport != tt.wantExport {
				t.Errorf("export = %q, want %q", gotExport, tt.wantExport)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -run TestResolveGoBinDir -v`
Expected: FAIL — `resolveGoBinDir` undefined

- [ ] **Step 3: Implement `resolveGoBinDir`**

Add to `cmd/setup.go` after the `isGoInstalled` function (after line 618):

```go
// resolveGoBinDir returns the Go bin directory and a portable PATH export line.
// It checks GOBIN first, then GOPATH/bin, then ~/go/bin.
// Paths under home use $HOME for portability; paths outside use absolute paths.
func resolveGoBinDir(home, gobin, gopath string) (dir string, exportLine string) {
	if gobin != "" {
		dir = gobin
	} else if gopath != "" {
		dir = filepath.Join(gopath, "bin")
	} else {
		dir = filepath.Join(home, "go", "bin")
	}

	// Build portable export line
	pathStr := dir
	if strings.HasPrefix(dir, home+string(filepath.Separator)) {
		pathStr = "$HOME" + dir[len(home):]
	}
	exportLine = fmt.Sprintf(`export PATH="%s:$PATH"`, pathStr)
	return dir, exportLine
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -run TestResolveGoBinDir -v`
Expected: PASS (all 6 sub-tests)

- [ ] **Step 5: Commit**

```bash
git add cmd/setup.go cmd/setup_uninstall_test.go
git commit -m "feat(setup): add resolveGoBinDir helper for Go bin PATH detection"
```

---

### Task 2: Wire `resolveGoBinDir` into `setupShellRc` and `runSetup`

**Files:**
- Modify: `cmd/setup.go:46-128` (`runSetup` and `setupShellRc`)
- Modify: `cmd/setup_uninstall_test.go` (add/update tests)

- [ ] **Step 1: Write failing test for setupShellRc with Go bin export**

Add to `cmd/setup_uninstall_test.go`:

```go
func TestSetupShellRcGoBinExport(t *testing.T) {
	tmp := t.TempDir()
	bashrc := filepath.Join(tmp, ".bashrc")
	os.WriteFile(bashrc, []byte("# my config\n"), 0644)

	setupShellRc(bashrc, filepath.Join(tmp, ".local", "bin"), filepath.Join(tmp, ".rt"),
		false, `export PATH="$HOME/go/bin:$PATH"`, "hook.bash", ".bashrc")

	result, _ := os.ReadFile(bashrc)
	lines := string(result)

	if !strings.Contains(lines, `export PATH="$HOME/go/bin:$PATH"`) {
		t.Error("Go bin PATH export not added")
	}
	if !strings.Contains(lines, "source $HOME/.rt/hook.bash") {
		t.Error("hook source line not added")
	}
}

func TestSetupShellRcGoBinExportAlreadyPresent(t *testing.T) {
	tmp := t.TempDir()
	bashrc := filepath.Join(tmp, ".bashrc")
	initial := strings.Join([]string{
		"# my config",
		`export PATH="$HOME/go/bin:$PATH"`,
		"",
		"# Red Team Operation Logger",
		"source $HOME/.rt/hook.bash",
	}, "\n")
	os.WriteFile(bashrc, []byte(initial), 0644)

	setupShellRc(bashrc, filepath.Join(tmp, ".local", "bin"), filepath.Join(tmp, ".rt"),
		false, `export PATH="$HOME/go/bin:$PATH"`, "hook.bash", ".bashrc")

	result, _ := os.ReadFile(bashrc)
	if string(result) != initial {
		t.Errorf("setupShellRc modified file when Go bin export already present:\n%s", result)
	}
}

func TestSetupShellRcNoGoBinExport(t *testing.T) {
	// When goBinExportLine is empty, no Go bin export should be added
	tmp := t.TempDir()
	bashrc := filepath.Join(tmp, ".bashrc")
	os.WriteFile(bashrc, []byte("# my config\n"), 0644)

	setupShellRc(bashrc, filepath.Join(tmp, ".local", "bin"), filepath.Join(tmp, ".rt"),
		false, "", "hook.bash", ".bashrc")

	result, _ := os.ReadFile(bashrc)
	if strings.Contains(string(result), "go/bin") {
		t.Error("Go bin export added when goBinExportLine was empty")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -run 'TestSetupShellRcGoBin|TestSetupShellRcNoGoBin' -v`
Expected: FAIL — `setupShellRc` has wrong number of arguments

- [ ] **Step 3: Update `setupShellRc` signature and implementation**

In `cmd/setup.go`, change the `setupShellRc` function signature (line 416) and add Go bin detection/insertion logic:

Change from:
```go
func setupShellRc(rcFile, localBin, rtDir string, addPathExport bool, hookFile, rcName string) {
```

To:
```go
func setupShellRc(rcFile, localBin, rtDir string, addPathExport bool, goBinExportLine, hookFile, rcName string) {
```

Add inside the line-scanning loop (after the `hasSourceLine` check around line 442), a new variable `hasGoBinExport` and detection:

Before the loop, add:
```go
hasGoBinExport := false
```

Inside the loop, after the `hasSourceLine` check:
```go
// Check for existing Go bin PATH export
if goBinExportLine != "" && !strings.HasPrefix(trimmed, "#") && trimmed == goBinExportLine {
    hasGoBinExport = true
}
```

Also change the existing `else` clause of the `addPathExport` block (line 455) to suppress the "skipping PATH export" message when Go bin export will be added:

Change from:
```go
} else {
    fmt.Printf("[ok] Binary on PATH, skipping PATH export in %s\n", rcName)
}
```

To:
```go
} else if goBinExportLine == "" {
    fmt.Printf("[ok] Binary on PATH, skipping PATH export in %s\n", rcName)
}
```

Then after the entire `addPathExport` block, add:
```go
// Append Go bin PATH export if needed
if goBinExportLine != "" {
    if !hasGoBinExport {
        newLines = append(newLines, "", goBinExportLine)
        fmt.Printf("[+]  Added Go bin to PATH in %s\n", rcName)
    } else {
        fmt.Printf("[ok] Go bin already in PATH\n")
    }
}
```

- [ ] **Step 4: Update all `setupShellRc` call sites in `runSetup`**

In `runSetup` (lines 119-123), update the calls to pass the new `goBinExportLine` parameter. Add a variable after the switch block:

After line 79 (`addPathExport := false`), add:
```go
goBinExportLine := ""
```

In the `installGoInstall` case (after line 84), add:
```go
_, goBinExportLine = resolveGoBinDir(home, os.Getenv("GOBIN"), os.Getenv("GOPATH"))
```

Update the two `setupShellRc` calls (lines 120, 123):
```go
setupShellRc(zshrc, localBin, rtDir, addPathExport, goBinExportLine, "hook.zsh", ".zshrc")
```
```go
setupShellRc(bashrc, localBin, rtDir, addPathExport, goBinExportLine, "hook.bash", ".bashrc")
```

- [ ] **Step 5: Fix existing tests that call `setupShellRc` with old signature**

Update `TestSetupShellRcBash` and `TestSetupShellRcIdempotent` in `cmd/setup_uninstall_test.go` to pass the new `goBinExportLine` parameter (empty string `""`) as the 5th argument:

`TestSetupShellRcBash` (line 227):
```go
setupShellRc(bashrc, filepath.Join(tmp, ".local", "bin"), filepath.Join(tmp, ".rt"), false, "", "hook.bash", ".bashrc")
```

`TestSetupShellRcIdempotent` (line 252):
```go
setupShellRc(bashrc, filepath.Join(tmp, ".local", "bin"), filepath.Join(tmp, ".rt"), false, "", "hook.bash", ".bashrc")
```

- [ ] **Step 6: Run all tests**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add cmd/setup.go cmd/setup_uninstall_test.go
git commit -m "feat(setup): add Go bin PATH export to setupShellRc"
```

---

### Task 3: Update uninstall to remove Go bin PATH exports

**Files:**
- Modify: `cmd/uninstall.go:107-184` (`uninstallCleanShellRc`)
- Modify: `cmd/setup_uninstall_test.go` (add tests)

- [ ] **Step 1: Write failing test for uninstall removing Go bin export**

Add to `cmd/setup_uninstall_test.go`:

```go
func TestUninstallCleansGoBinExport(t *testing.T) {
	tmp := t.TempDir()
	zshrc := filepath.Join(tmp, ".zshrc")

	content := strings.Join([]string{
		"# my config",
		`export PATH="$HOME/go/bin:$PATH"`,
		"# Red Team Operation Logger",
		"source $HOME/.rt/hook.zsh",
		`export PATH="$HOME/.local/bin:$PATH"`,
		"export EDITOR=vim",
	}, "\n")
	os.WriteFile(zshrc, []byte(content), 0644)

	uninstallCleanShellRc(zshrc, ".rt/hook.zsh", ".zshrc")

	result, _ := os.ReadFile(zshrc)
	lines := string(result)

	if strings.Contains(lines, `$HOME/go/bin`) {
		t.Error("Go bin PATH export was not removed")
	}
	if strings.Contains(lines, `$HOME/.local/bin`) {
		t.Error("~/.local/bin PATH export was not removed")
	}
	if !strings.Contains(lines, "export EDITOR=vim") {
		t.Error("unrelated export was incorrectly removed")
	}
}

func TestUninstallCleansCustomGoBinExport(t *testing.T) {
	tmp := t.TempDir()
	zshrc := filepath.Join(tmp, ".zshrc")

	content := strings.Join([]string{
		"# my config",
		`export PATH="/opt/gowork/bin:$PATH"`,
		"# Red Team Operation Logger",
		"source $HOME/.rt/hook.zsh",
		"alias ls='ls -la'",
	}, "\n")
	os.WriteFile(zshrc, []byte(content), 0644)

	uninstallCleanShellRc(zshrc, ".rt/hook.zsh", ".zshrc")

	result, _ := os.ReadFile(zshrc)
	lines := string(result)

	// Custom Go bin PATH — uninstall can't know this was added by rtlog,
	// so it should NOT be removed (only the default $HOME/go/bin pattern)
	if !strings.Contains(lines, `/opt/gowork/bin`) {
		t.Error("custom PATH export was incorrectly removed")
	}
	if !strings.Contains(lines, "alias ls='ls -la'") {
		t.Error("alias was incorrectly removed")
	}
}
```

- [ ] **Step 2: Run tests to verify the first fails**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -run 'TestUninstallCleansGoBinExport$' -v`
Expected: FAIL — `$HOME/go/bin` export not removed

- [ ] **Step 3: Update `uninstallCleanShellRc` to remove Go bin export**

In `cmd/uninstall.go`, inside the `uninstallCleanShellRc` function's line-scanning loop, after the existing PATH export removal (after line 141), add:

```go
// Remove Go bin PATH export added by setup (default path)
if trimmed == `export PATH="$HOME/go/bin:$PATH"` {
    removed = true
    continue
}
```

- [ ] **Step 4: Run all tests**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/uninstall.go cmd/setup_uninstall_test.go
git commit -m "fix(uninstall): remove Go bin PATH export from shell rc files"
```

---

### Task 4: Manual verification and final commit

- [ ] **Step 1: Run the full test suite**

Run: `cd /home/cyb3r/RTLog && go test ./...`
Expected: ALL PASS

- [ ] **Step 2: Build and verify**

Run: `cd /home/cyb3r/RTLog && go build -o /dev/null .`
Expected: Clean build, no errors

- [ ] **Step 3: Verify setup --help still shows correct info**

Run: `cd /home/cyb3r/RTLog && go run . setup --help`
Expected: Help text displays correctly
