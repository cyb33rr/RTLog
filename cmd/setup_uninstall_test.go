package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsZshShell(t *testing.T) {
	tests := []struct {
		shell string
		want  bool
	}{
		{"/usr/bin/zsh", true},
		{"/bin/zsh", true},
		{"/bin/bash", false},
		{"/usr/bin/fish", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			os.Setenv("SHELL", tt.shell)
			defer os.Unsetenv("SHELL")
			if got := isZshShell(); got != tt.want {
				t.Errorf("isZshShell() with SHELL=%q = %v, want %v", tt.shell, got, tt.want)
			}
		})
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

	uninstallCleanZshrc(zshrc)

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

	uninstallCleanZshrc(zshrc)

	result, _ := os.ReadFile(zshrc)
	// Should not have 3+ consecutive newlines (i.e. 2+ blank lines)
	if strings.Contains(string(result), "\n\n\n") {
		t.Errorf("consecutive blank lines remain:\n%s", result)
	}
}
