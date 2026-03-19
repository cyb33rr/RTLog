package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileExists(t *testing.T) {
	tmp := t.TempDir()

	// Existing file
	f := filepath.Join(tmp, "exists")
	os.WriteFile(f, []byte("x"), 0644)
	if !fileExists(f) {
		t.Error("fileExists returned false for existing file")
	}

	// Non-existent file
	if fileExists(filepath.Join(tmp, "nope")) {
		t.Error("fileExists returned true for missing file")
	}

	// Directory should return false
	if fileExists(tmp) {
		t.Error("fileExists returned true for directory")
	}
}

func TestCollapseBlankLines(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "no blanks",
			input: []string{"a", "b", "c"},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "single blank preserved",
			input: []string{"a", "", "b"},
			want:  []string{"a", "", "b"},
		},
		{
			name:  "consecutive blanks collapsed",
			input: []string{"a", "", "", "", "b"},
			want:  []string{"a", "", "b"},
		},
		{
			name:  "multiple groups",
			input: []string{"a", "", "", "b", "", "", "c"},
			want:  []string{"a", "", "b", "", "c"},
		},
		{
			name:  "whitespace-only lines treated as blank",
			input: []string{"a", "", "  ", "\t", "b"},
			want:  []string{"a", "", "b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collapseBlankLines(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d\ngot:  %q\nwant: %q", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSetupSymlinkReturnValue(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	os.WriteFile(target, []byte("binary"), 0755)

	// Case 1: no file at link path — should create and return true
	link := filepath.Join(tmp, "link")
	got := setupSymlink(link, target)
	if !got {
		t.Error("setupSymlink returned false for new symlink")
	}

	// Case 2: regular file blocking — should return false
	blocker := filepath.Join(tmp, "blocker")
	os.WriteFile(blocker, []byte("not a symlink"), 0644)
	got = setupSymlink(blocker, target)
	if got {
		t.Error("setupSymlink returned true when blocked by regular file")
	}
}

func TestUninstallNarrowHookMatching(t *testing.T) {
	tmp := t.TempDir()
	zshrc := filepath.Join(tmp, ".zshrc")

	content := strings.Join([]string{
		"# some config",
		"source ~/.config/hook.zsh",
		"# Red Team Operation Logger",
		"source $HOME/.rt/hook.zsh",
		`export PATH="$HOME/.local/bin:$PATH"`,
		"alias ll='ls -la'",
	}, "\n")
	os.WriteFile(zshrc, []byte(content), 0644)

	uninstallCleanShellRc(zshrc, ".rt/hook.zsh", ".zshrc")

	result, _ := os.ReadFile(zshrc)
	lines := string(result)

	// Our lines should be removed
	if strings.Contains(lines, ".rt/hook.zsh") {
		t.Error(".rt/hook.zsh line was not removed")
	}
	if strings.Contains(lines, `export PATH="$HOME/.local/bin:$PATH"`) {
		t.Error("PATH export was not removed")
	}

	// Other hook.zsh should survive
	if !strings.Contains(lines, "source ~/.config/hook.zsh") {
		t.Error("non-rtlog hook.zsh line was incorrectly removed")
	}

	// No consecutive blank lines
	if strings.Contains(lines, "\n\n\n") {
		t.Error("consecutive blank lines remain after uninstall")
	}
}

func TestUninstallBlankLineCollapsing(t *testing.T) {
	tmp := t.TempDir()
	zshrc := filepath.Join(tmp, ".zshrc")

	// Simulate what setup creates — blank separator lines around our content
	content := strings.Join([]string{
		"# existing config",
		"",
		`export PATH="$HOME/.local/bin:$PATH"`,
		"",
		"# Red Team Operation Logger",
		"source $HOME/.rt/hook.zsh",
		"",
		"# more config",
	}, "\n")
	os.WriteFile(zshrc, []byte(content), 0644)

	uninstallCleanShellRc(zshrc, ".rt/hook.zsh", ".zshrc")

	result, _ := os.ReadFile(zshrc)
	// Should not have 3+ consecutive newlines (i.e. 2+ blank lines)
	if strings.Contains(string(result), "\n\n\n") {
		t.Errorf("consecutive blank lines remain:\n%s", result)
	}
}

func TestUninstallCleanBashrc(t *testing.T) {
	tmp := t.TempDir()
	bashrc := filepath.Join(tmp, ".bashrc")

	content := strings.Join([]string{
		"# existing bash config",
		"alias ls='ls --color=auto'",
		"# Red Team Operation Logger",
		"source $HOME/.rt/hook.bash",
		`export PATH="$HOME/.local/bin:$PATH"`,
		"export EDITOR=vim",
	}, "\n")
	os.WriteFile(bashrc, []byte(content), 0644)

	uninstallCleanShellRc(bashrc, ".rt/hook.bash", ".bashrc")

	result, _ := os.ReadFile(bashrc)
	lines := string(result)

	if strings.Contains(lines, ".rt/hook.bash") {
		t.Error(".rt/hook.bash line was not removed")
	}
	if strings.Contains(lines, `export PATH="$HOME/.local/bin:$PATH"`) {
		t.Error("PATH export was not removed from .bashrc")
	}
	if !strings.Contains(lines, "alias ls='ls --color=auto'") {
		t.Error("existing bash config was incorrectly removed")
	}
	if !strings.Contains(lines, "export EDITOR=vim") {
		t.Error("other export was incorrectly removed")
	}
}

func TestUninstallBashrcDoesNotRemoveZshHook(t *testing.T) {
	tmp := t.TempDir()
	bashrc := filepath.Join(tmp, ".bashrc")

	// .bashrc that somehow has a zsh hook line — should NOT be removed by bash pattern
	content := strings.Join([]string{
		"source $HOME/.rt/hook.zsh",
		"source $HOME/.rt/hook.bash",
	}, "\n")
	os.WriteFile(bashrc, []byte(content), 0644)

	uninstallCleanShellRc(bashrc, ".rt/hook.bash", ".bashrc")

	result, _ := os.ReadFile(bashrc)
	lines := string(result)

	if !strings.Contains(lines, ".rt/hook.zsh") {
		t.Error("zsh hook line was incorrectly removed by bash uninstall")
	}
	if strings.Contains(lines, ".rt/hook.bash") {
		t.Error(".rt/hook.bash line was not removed")
	}
}

func TestSetupShellRcBash(t *testing.T) {
	tmp := t.TempDir()
	bashrc := filepath.Join(tmp, ".bashrc")

	// Create an existing .bashrc
	os.WriteFile(bashrc, []byte("# my bash config\n"), 0644)

	setupShellRc(bashrc, "", "hook.bash", ".bashrc")

	result, _ := os.ReadFile(bashrc)
	lines := string(result)

	if !strings.Contains(lines, "source $HOME/.rt/hook.bash") {
		t.Error("hook.bash source line not added to .bashrc")
	}
	if !strings.Contains(lines, "# Red Team Operation Logger") {
		t.Error("comment not added to .bashrc")
	}
}

func TestSetupShellRcIdempotent(t *testing.T) {
	tmp := t.TempDir()
	bashrc := filepath.Join(tmp, ".bashrc")

	initial := strings.Join([]string{
		"# my bash config",
		"",
		"# Red Team Operation Logger",
		"source $HOME/.rt/hook.bash",
	}, "\n")
	os.WriteFile(bashrc, []byte(initial), 0644)

	setupShellRc(bashrc, "", "hook.bash", ".bashrc")

	result, _ := os.ReadFile(bashrc)
	if string(result) != initial {
		t.Errorf("setupShellRc modified already-configured file:\n%s", result)
	}
}

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

func TestSetupCleanup_MissingFiles(t *testing.T) {
	// Cleanup should not error on a fresh directory with no files to delete
	rtDir := t.TempDir()
	setupCleanup(rtDir) // should not panic or error
}

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
	os.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".rt"), 0755)
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
	binPath := filepath.Join(rtDir, "rtlog")
	os.WriteFile(binPath, []byte("binary"), 0755)
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
	realBin := filepath.Join(rtDir, "rtlog")
	os.WriteFile(realBin, []byte("binary"), 0755)
	os.Symlink(realBin, filepath.Join(localBin, "rtlog"))
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

func TestSetupCopySelfTo_PermissionError(t *testing.T) {
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

func TestSetupShellRcGoBinExport(t *testing.T) {
	tmp := t.TempDir()
	bashrc := filepath.Join(tmp, ".bashrc")
	os.WriteFile(bashrc, []byte("# my config\n"), 0644)

	setupShellRc(bashrc, `export PATH="$HOME/go/bin:$PATH"`, "hook.bash", ".bashrc")

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

	setupShellRc(bashrc, `export PATH="$HOME/go/bin:$PATH"`, "hook.bash", ".bashrc")

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

	setupShellRc(bashrc, "", "hook.bash", ".bashrc")

	result, _ := os.ReadFile(bashrc)
	if strings.Contains(string(result), "go/bin") {
		t.Error("Go bin export added when goBinExportLine was empty")
	}
}

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
			gotDir, gotExport := resolveGoBinDir(home, tt.gopath, tt.gobin)
			if gotDir != tt.wantDir {
				t.Errorf("dir = %q, want %q", gotDir, tt.wantDir)
			}
			if gotExport != tt.wantExport {
				t.Errorf("export = %q, want %q", gotExport, tt.wantExport)
			}
		})
	}
}

func TestSetupMigrateSymlink(t *testing.T) {
	tmp := t.TempDir()
	localBin := filepath.Join(tmp, ".local", "bin")
	os.MkdirAll(localBin, 0755)
	rtBinary := filepath.Join(tmp, ".rt", "rtlog")

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

	link := filepath.Join(localBin, "rtlog")
	os.Symlink("/usr/local/bin/rtlog", link)

	setupMigrateSymlink(link, filepath.Join(tmp, ".rt", "rtlog"))

	if _, err := os.Lstat(link); err != nil {
		t.Error("non-matching symlink was incorrectly removed")
	}
}

func TestSetupMigrateSymlink_RegularFile(t *testing.T) {
	tmp := t.TempDir()
	localBin := filepath.Join(tmp, ".local", "bin")
	os.MkdirAll(localBin, 0755)

	file := filepath.Join(localBin, "rtlog")
	os.WriteFile(file, []byte("binary"), 0755)

	setupMigrateSymlink(file, filepath.Join(tmp, ".rt", "rtlog"))

	if _, err := os.Stat(file); err != nil {
		t.Error("regular file was incorrectly removed")
	}
}

func TestSetupMigrateSymlink_NotExists(t *testing.T) {
	setupMigrateSymlink("/nonexistent/path", "/also/nonexistent")
}

func TestSetupShellRcMigratesLocalBinExport(t *testing.T) {
	tmp := t.TempDir()
	bashrc := filepath.Join(tmp, ".bashrc")

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

	if strings.Contains(lines, `$HOME/.local/bin`) {
		t.Error("old ~/.local/bin PATH export was not removed")
	}
	if !strings.Contains(lines, `$HOME/go/bin`) {
		t.Error("Go bin PATH export not added")
	}
	if !strings.Contains(lines, "source $HOME/.rt/hook.bash") {
		t.Error("hook source line was removed")
	}
}
