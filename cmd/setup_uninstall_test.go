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

	setupShellRc(bashrc, filepath.Join(tmp, ".local", "bin"), filepath.Join(tmp, ".rt"), false, "hook.bash", ".bashrc")

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

	setupShellRc(bashrc, filepath.Join(tmp, ".local", "bin"), filepath.Join(tmp, ".rt"), false, "hook.bash", ".bashrc")

	result, _ := os.ReadFile(bashrc)
	if string(result) != initial {
		t.Errorf("setupShellRc modified already-configured file:\n%s", result)
	}
}
