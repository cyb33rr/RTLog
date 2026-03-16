package cmd

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// embeddedFS holds the embedded hook files, tools.conf, and extract.conf.
var embeddedFS embed.FS

// SetEmbeddedFiles injects the embedded filesystem from main.
func SetEmbeddedFiles(fs embed.FS) {
	embeddedFS = fs
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install rtlog into ~/.rt/ and configure the shell",
	Long: `Idempotent setup that installs rtlog to ~/.rt/ and configures zsh and/or bash.

Steps performed:
  1. Create ~/.rt/logs/ and ~/.local/bin/
  2. Write embedded hook files (interactive + non-interactive) and config to ~/.rt/
  3. Copy this binary to ~/.rt/rtlog  (skipped if already on PATH)
  4. Create symlink ~/.local/bin/rtlog  (skipped if already on PATH)
  5. Configure ~/.zshrc and/or ~/.bashrc (hook source line; PATH export)
  6. Configure ~/.zshenv for non-interactive zsh capture
  7. Export BASH_ENV in shell rc files for non-interactive bash capture`,
	Args: cobra.NoArgs,
	Run:  runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

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

	// 1. Create directories
	setupCreateDir(logDir)
	setupCreateDir(localBin)

	// 2. Write embedded files (both shells, always)
	setupWriteEmbedded("hook.zsh", filepath.Join(rtDir, "hook.zsh"), false)
	setupWriteEmbedded("hook.bash", filepath.Join(rtDir, "hook.bash"), false)
	setupWriteEmbedded("bash-preexec.sh", filepath.Join(rtDir, "bash-preexec.sh"), false)
	setupWriteEmbedded("tools.conf", filepath.Join(rtDir, "tools.conf"), true)
	setupWriteEmbedded("extract.conf", filepath.Join(rtDir, "extract.conf"), true)
	setupWriteEmbedded("hook-noninteractive.zsh", filepath.Join(rtDir, "hook-noninteractive.zsh"), false)
	setupWriteEmbedded("hook-noninteractive.bash", filepath.Join(rtDir, "hook-noninteractive.bash"), false)

	// 3-4. Copy binary + symlink (skip if already on PATH, e.g. go install)
	onPath := isOnPath()
	addPathExport := false
	if !onPath {
		setupCopySelf(filepath.Join(rtDir, "rtlog"))
		addPathExport = setupSymlink(filepath.Join(localBin, "rtlog"), filepath.Join(rtDir, "rtlog"))
	} else {
		fmt.Println("[ok] Binary already on PATH, skipping copy and symlink")
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

// setupCreateDir creates a directory if it doesn't exist.
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

// setupWriteEmbedded writes an embedded file to dst, skipping if identical.
// If userConfig is true and the file already exists with different content,
// the user is prompted before overwriting.
func setupWriteEmbedded(name, dst string, userConfig bool) {
	data, err := embeddedFS.ReadFile(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Embedded file %s not found: %v\n", name, err)
		os.Exit(1)
	}

	// Check if existing file is identical
	existing, err := os.ReadFile(dst)
	if err == nil && bytes.Equal(existing, data) {
		fmt.Printf("[ok] %s is up to date\n", name)
		return
	}

	// For user-editable config files, prompt before overwriting
	if userConfig && err == nil {
		fmt.Printf("[?]  %s has been modified. Overwrite with defaults? [y/N] ", name)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Printf("[ok] Keeping existing %s\n", name)
			return
		}
	}

	// Atomic write: temp file + rename
	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, "."+name+".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to create temp file for %s: %v\n", name, err)
		os.Exit(1)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		fmt.Fprintf(os.Stderr, "[!]  Failed to write %s: %v\n", name, err)
		os.Exit(1)
	}
	if err := tmp.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to close %s: %v\n", name, err)
		os.Exit(1)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to install %s: %v\n", name, err)
		os.Exit(1)
	}
	fmt.Printf("[+]  Installed %s -> %s\n", name, dst)
}

// setupCopySelf copies the running binary to dst using atomic temp+rename.
func setupCopySelf(dst string) {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Cannot determine own path: %v\n", err)
		os.Exit(1)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Cannot resolve own path: %v\n", err)
		os.Exit(1)
	}

	// If we're already at the destination, skip
	absDst, _ := filepath.Abs(dst)
	if self == absDst {
		fmt.Printf("[ok] Binary already at %s\n", dst)
		return
	}

	// Check if existing binary is identical
	selfInfo, err := os.Stat(self)
	if err == nil {
		dstInfo, derr := os.Stat(dst)
		if derr == nil && selfInfo.Size() == dstInfo.Size() {
			// Quick size check — if sizes match, compare content
			selfData, e1 := os.ReadFile(self)
			dstData, e2 := os.ReadFile(dst)
			if e1 == nil && e2 == nil && bytes.Equal(selfData, dstData) {
				fmt.Printf("[ok] Binary is up to date: %s\n", dst)
				return
			}
		}
	}

	// Atomic copy: write to temp + rename
	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, ".rtlog.")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to create temp file: %v\n", err)
		os.Exit(1)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	src, err := os.Open(self)
	if err != nil {
		tmp.Close()
		fmt.Fprintf(os.Stderr, "[!]  Cannot open self: %v\n", err)
		os.Exit(1)
	}
	defer src.Close()

	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		fmt.Fprintf(os.Stderr, "[!]  Failed to copy binary: %v\n", err)
		os.Exit(1)
	}
	if err := tmp.Chmod(0755); err != nil {
		tmp.Close()
		fmt.Fprintf(os.Stderr, "[!]  Failed to set permissions: %v\n", err)
		os.Exit(1)
	}
	if err := tmp.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to close temp file: %v\n", err)
		os.Exit(1)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to install binary: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[+]  Installed binary: %s\n", dst)
}

// setupSymlink creates or updates a symlink.
// Returns true if the symlink is in place, false if blocked by a regular file.
func setupSymlink(link, target string) bool {
	updated := false
	if existing, err := os.Readlink(link); err == nil {
		if existing == target {
			fmt.Printf("[ok] Symlink exists: %s -> %s\n", link, target)
			return true
		}
		os.Remove(link)
		updated = true
	} else if _, err := os.Stat(link); err == nil {
		fmt.Printf("[!]  %s exists but is not a symlink, skipping\n", link)
		return false
	}

	if err := os.Symlink(target, link); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to create symlink: %v\n", err)
		os.Exit(1)
	}
	if updated {
		fmt.Printf("[+]  Updated symlink: %s -> %s\n", link, target)
	} else {
		fmt.Printf("[+]  Created symlink: %s -> %s\n", link, target)
	}
	return true
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

// isOnPath checks if the running binary's directory is already in $PATH.
func isOnPath() bool {
	self, err := os.Executable()
	if err != nil {
		return false
	}
	self, _ = filepath.EvalSymlinks(self)
	selfDir := filepath.Dir(self)
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		abs, err := filepath.Abs(dir)
		if err == nil && abs == selfDir {
			return true
		}
	}
	return false
}

// setupShellRc ensures PATH and hook source lines are in the given rc file.
// hookFile is "hook.zsh" or "hook.bash". rcName is ".zshrc" or ".bashrc" (for messages).
func setupShellRc(rcFile, localBin, rtDir string, addPathExport bool, hookFile, rcName string) {
	sourceLine := fmt.Sprintf("source %s/.rt/%s", "$HOME", hookFile)

	// Read existing content
	content, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "[!]  Cannot read %s: %v\n", rcFile, err)
		os.Exit(1)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	hasPathExport := false
	hasSourceLine := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for existing PATH export (exclude commented lines)
		if !strings.HasPrefix(trimmed, "#") && strings.HasPrefix(trimmed, `export PATH="$HOME/.local/bin`) {
			hasPathExport = true
		}

		// Check for our source line
		if trimmed == sourceLine {
			hasSourceLine = true
		}

		newLines = append(newLines, line)
	}

	// Append PATH export if missing and needed (only when symlink was created)
	if addPathExport {
		if !hasPathExport {
			newLines = append(newLines, "", `export PATH="$HOME/.local/bin:$PATH"`)
			fmt.Printf("[+]  Added %s to PATH in %s\n", localBin, rcName)
		} else {
			fmt.Printf("[ok] %s already in PATH\n", localBin)
		}
	} else {
		fmt.Printf("[ok] Binary on PATH, skipping PATH export in %s\n", rcName)
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
		return
	}

	dir := filepath.Dir(rcFile)
	tmp, err := os.CreateTemp(dir, "."+rcName+".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to create temp file: %v\n", err)
		os.Exit(1)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.WriteString(newContent); err != nil {
		tmp.Close()
		fmt.Fprintf(os.Stderr, "[!]  Failed to write %s: %v\n", rcName, err)
		os.Exit(1)
	}
	// Preserve original permissions
	if info, err := os.Stat(rcFile); err == nil {
		tmp.Chmod(info.Mode())
	} else {
		tmp.Chmod(0644)
	}
	if err := tmp.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to close temp file: %v\n", err)
		os.Exit(1)
	}
	if err := os.Rename(tmpName, rcFile); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to update %s: %v\n", rcName, err)
		os.Exit(1)
	}
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
