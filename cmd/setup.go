package cmd

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// rtlogTag is appended to lines written by setup for safe identification during uninstall.
const rtlogTag = "  # added by rtlog"

// isLegacySourceLine returns true if the line is an old repo-based or python-hook source line
// that should be removed during setup migration or uninstall.
func isLegacySourceLine(trimmed string) bool {
	return strings.Contains(trimmed, "source") &&
		(strings.Contains(trimmed, "/rtlog/hook.zsh") || strings.Contains(trimmed, "/python-hook/hook.zsh")) &&
		!strings.Contains(trimmed, ".rt/hook.")
}

// embeddedFS holds the embedded hook files, tools.conf, and extract.conf.
var embeddedFS embed.FS

// SetEmbeddedFiles injects the embedded filesystem from main.
func SetEmbeddedFiles(fs embed.FS) {
	embeddedFS = fs
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure rtlog shell hooks and environment",
	Long: `Idempotent setup that configures zsh and/or bash for rtlog.

Steps performed:
  1. Create ~/.rt/logs/
  2. Clean up stale files from previous versions
  3. Migrate old installs (remove ~/.rt/rtlog binary, ~/.local/bin symlink)
  4. Write embedded hook files (interactive + non-interactive) and config to ~/.rt/
  5. Configure ~/.zshrc and/or ~/.bashrc (hook source line)
  6. Configure ~/.zshenv for non-interactive zsh capture
  7. Export BASH_ENV in shell rc files for non-interactive bash capture`,
	Args: cobra.NoArgs,
	Run:  runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

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

	// 3. Migrate old installs
	setupMigrateSymlink(
		filepath.Join(home, ".local", "bin", "rtlog"),
		filepath.Join(rtDir, "rtlog"),
	)

	// 4. Write embedded files
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

	// 5. Configure shell rc files
	zshrcExists := fileExists(zshrc)
	bashrcExists := fileExists(bashrc)

	if zshrcExists {
		if err := setupShellRc(zshrc, "hook.zsh", ".zshrc"); err != nil {
			return err
		}
	}
	if bashrcExists {
		if err := setupShellRc(bashrc, "hook.bash", ".bashrc"); err != nil {
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

	if _, err := exec.LookPath("rtlog"); err != nil {
		fmt.Println("[!]  Warning: rtlog is not on your PATH — hooks won't work until you add it.")
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

// setupCreateDir creates a directory if it doesn't exist.
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

// setupCleanup deletes stale application-managed files from rtDir.
// Preserved: logs/, state, tools.conf, extract.conf.
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
		"uninstall.sh",
	}
	for _, name := range denylist {
		path := filepath.Join(rtDir, name)
		if err := os.Remove(path); err == nil {
			fmt.Printf("[~]  Cleaned up: %s\n", name)
		}
	}

	// Clean orphan temp files from shell hooks
	for _, pattern := range []string{"/tmp/.rtlog_out.*", "/tmp/.rtlog_ni_out.*"} {
		matches, _ := filepath.Glob(pattern)
		for _, m := range matches {
			os.Remove(m) // errors silently ignored (e.g. EPERM on shared systems)
		}
	}
}

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

// setupWriteEmbedded writes an embedded file to dst, skipping if identical.
// If userConfig is true and the file already exists with different content,
// the user is prompted before overwriting.
func setupWriteEmbedded(name, dst string, userConfig bool) error {
	data, err := embeddedFS.ReadFile(name)
	if err != nil {
		return fmt.Errorf("embedded file %s not found: %w", name, err)
	}

	// Check if existing file is identical
	existing, err := os.ReadFile(dst)
	if err == nil && bytes.Equal(existing, data) {
		fmt.Printf("[ok] %s is up to date\n", name)
		return nil
	}

	// For user-editable config files, prompt before overwriting
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

	// Atomic write: temp file + rename
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

// collapseBlankLines reduces consecutive blank lines to a single blank line.
func collapseBlankLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	prevBlank := false
	for _, line := range lines {
		blank := strings.TrimSpace(line) == ""
		if blank && prevBlank {
			continue
		}
		out = append(out, line)
		prevBlank = blank
	}
	return out
}

// fileExists checks if a file exists (not a directory).
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// setupShellRc ensures the hook source line is in the given rc file and removes legacy PATH exports.
// hookFile is "hook.zsh" or "hook.bash". rcName is ".zshrc" or ".bashrc" (for messages).
func setupShellRc(rcFile, hookFile, rcName string) error {
	sourceLine := fmt.Sprintf("source %s/.rt/%s", "$HOME", hookFile)

	content, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot read %s: %w", rcFile, err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	hasSourceLine := false
	migrated := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Migration: remove old ~/.local/bin PATH export
		if !strings.HasPrefix(trimmed, "#") && trimmed == `export PATH="$HOME/.local/bin:$PATH"` {
			migrated = true
			continue
		}

		// Migration: remove Go bin PATH export (tagged lines, any path)
		if strings.Contains(trimmed, "export PATH=") && strings.HasSuffix(trimmed, rtlogTag) {
			migrated = true
			continue
		}

		// Migration: remove untagged default Go bin PATH export (backward compat)
		if trimmed == `export PATH="$HOME/go/bin:$PATH"` {
			migrated = true
			continue
		}

		// Migration: remove old repo-based or python-hook source lines
		if isLegacySourceLine(trimmed) {
			migrated = true
			continue
		}

		// Check for our source line
		if trimmed == sourceLine {
			hasSourceLine = true
		}

		newLines = append(newLines, line)
	}

	if migrated {
		newLines = collapseBlankLines(newLines)
		fmt.Printf("[~]  Migrated old config lines in %s\n", rcName)
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

// setupZshenv ensures the non-interactive zsh hook is sourced from ~/.zshenv.
func setupZshenv(zshenvPath, rtDir string) {
	sourceLine := fmt.Sprintf("source %s/.rt/hook-noninteractive.zsh", "$HOME")

	content, err := os.ReadFile(zshenvPath)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "[!]  Cannot read .zshenv: %v\n", err)
		return
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == sourceLine {
			fmt.Println("[ok] Non-interactive zsh hook already in .zshenv")
			return
		}
	}

	lines = append(lines, "", "# RTLog non-interactive capture", sourceLine)
	lines = collapseBlankLines(lines)
	newContent := strings.Join(lines, "\n")

	if string(content) == newContent {
		return
	}

	dir := filepath.Dir(zshenvPath)
	tmp, err := os.CreateTemp(dir, ".zshenv.")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to create temp file: %v\n", err)
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.WriteString(newContent); err != nil {
		tmp.Close()
		fmt.Fprintf(os.Stderr, "[!]  Failed to write .zshenv: %v\n", err)
		return
	}
	if info, err := os.Stat(zshenvPath); err == nil {
		tmp.Chmod(info.Mode())
	} else {
		tmp.Chmod(0644)
	}
	if err := tmp.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to close temp file: %v\n", err)
		return
	}
	if err := os.Rename(tmpName, zshenvPath); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to update .zshenv: %v\n", err)
		return
	}
	fmt.Println("[+]  Added non-interactive hook to .zshenv")
}

// setupBashEnv ensures BASH_ENV is exported in the given rc file.
func setupBashEnv(rcFile, rtDir, rcName string) {
	exportLine := fmt.Sprintf(`export BASH_ENV="%s/.rt/hook-noninteractive.bash"`, "$HOME")

	content, err := os.ReadFile(rcFile)
	if err != nil {
		return // file doesn't exist, skip
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == exportLine {
			fmt.Printf("[ok] BASH_ENV already exported in %s\n", rcName)
			return
		}
	}

	lines = append(lines, "", "# RTLog non-interactive bash capture", exportLine)
	lines = collapseBlankLines(lines)
	newContent := strings.Join(lines, "\n")

	if string(content) == newContent {
		return
	}

	dir := filepath.Dir(rcFile)
	tmp, err := os.CreateTemp(dir, "."+rcName+".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to create temp file: %v\n", err)
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.WriteString(newContent); err != nil {
		tmp.Close()
		return
	}
	if info, err := os.Stat(rcFile); err == nil {
		tmp.Chmod(info.Mode())
	} else {
		tmp.Chmod(0644)
	}
	tmp.Close()
	os.Rename(tmpName, rcFile)
	fmt.Printf("[+]  Added BASH_ENV export to %s\n", rcName)
}

// resolveGoBinDir returns the Go bin directory and a portable PATH export line.
// It checks GOBIN first, then GOPATH/bin, then ~/go/bin.
// Paths under home use $HOME for portability; paths outside use absolute paths.
func resolveGoBinDir(home, gopath, gobin string) (dir string, exportLine string) {
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
	exportLine = fmt.Sprintf(`export PATH="%s:$PATH"%s`, pathStr, rtlogTag)
	return dir, exportLine
}
