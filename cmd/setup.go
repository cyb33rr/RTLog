package cmd

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// embeddedFS holds the embedded hook.zsh and tools.conf files.
var embeddedFS embed.FS

// SetEmbeddedFiles injects the embedded filesystem from main.
func SetEmbeddedFiles(fs embed.FS) {
	embeddedFS = fs
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install rtlog into ~/.rt/ and configure the shell",
	Long: `Idempotent setup that installs rtlog to ~/.rt/ and configures zsh.

Steps performed:
  1. Create ~/.rt/logs/ and ~/.local/bin/
  2. Write embedded hook.zsh and tools.conf to ~/.rt/
  3. Copy this binary to ~/.rt/rtlog  (skipped if already on PATH, e.g. go install)
  4. Create symlink ~/.local/bin/rtlog  (skipped if already on PATH)
  5. Configure ~/.zshrc (hook source line; PATH export only if symlink was created)`,
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

	fmt.Println("=== Red Team Operation Logger - Setup ===")
	fmt.Println()

	if !isZshShell() {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "unknown"
		}
		fmt.Printf("[!]  Current shell is %s — setup configures ~/.zshrc which may not be loaded\n", shell)
		fmt.Println()
	}

	// 1. Create directories
	setupCreateDir(logDir)
	setupCreateDir(localBin)

	// 2. Write embedded files
	setupWriteEmbedded("hook.zsh", filepath.Join(rtDir, "hook.zsh"))
	setupWriteEmbedded("tools.conf", filepath.Join(rtDir, "tools.conf"))

	// 3-4. Copy binary + symlink (skip if already on PATH, e.g. go install)
	onPath := isOnPath()
	addPathExport := false
	if !onPath {
		setupCopySelf(filepath.Join(rtDir, "rtlog"))
		addPathExport = setupSymlink(filepath.Join(localBin, "rtlog"), filepath.Join(rtDir, "rtlog"))
	} else {
		fmt.Println("[ok] Binary already on PATH, skipping copy and symlink")
	}

	// 5. Configure ~/.zshrc
	setupZshrc(zshrc, localBin, rtDir, addPathExport)

	fmt.Println()
	fmt.Println("=== Setup complete ===")
	fmt.Println()
	fmt.Println("Quick-start:")
	fmt.Println("  1. Reload shell:     source ~/.zshrc")
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
func setupWriteEmbedded(name, dst string) {
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

// isZshShell checks if the user's login shell is zsh.
func isZshShell() bool {
	return filepath.Base(os.Getenv("SHELL")) == "zsh"
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

// setupZshrc ensures PATH and hook source lines are in .zshrc.
// addPathExport controls whether the ~/.local/bin PATH export is added.
func setupZshrc(zshrc, localBin, rtDir string, addPathExport bool) {
	sourceLine := fmt.Sprintf("source %s/.rt/hook.zsh", "$HOME")

	// Read existing content
	content, err := os.ReadFile(zshrc)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "[!]  Cannot read %s: %v\n", zshrc, err)
		os.Exit(1)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	hasPathExport := false
	hasSourceLine := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for existing PATH export
		if strings.Contains(trimmed, `export PATH="$HOME/.local/bin`) {
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
			fmt.Printf("[+]  Added %s to PATH in .zshrc\n", localBin)
		} else {
			fmt.Printf("[ok] %s already in PATH\n", localBin)
		}
	} else {
		fmt.Println("[ok] Binary on PATH, skipping PATH export")
	}

	// Append source line if missing
	if !hasSourceLine {
		newLines = append(newLines, "", "# Red Team Operation Logger", sourceLine)
		fmt.Println("[+]  Added source line to .zshrc")
	} else {
		fmt.Println("[ok] hook.zsh already sourced in .zshrc")
	}

	// Atomic write
	newContent := strings.Join(newLines, "\n")
	if string(content) == newContent {
		return
	}

	dir := filepath.Dir(zshrc)
	tmp, err := os.CreateTemp(dir, ".zshrc.")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to create temp file: %v\n", err)
		os.Exit(1)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.WriteString(newContent); err != nil {
		tmp.Close()
		fmt.Fprintf(os.Stderr, "[!]  Failed to write .zshrc: %v\n", err)
		os.Exit(1)
	}
	// Preserve original permissions
	if info, err := os.Stat(zshrc); err == nil {
		tmp.Chmod(info.Mode())
	} else {
		tmp.Chmod(0644)
	}
	if err := tmp.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to close temp file: %v\n", err)
		os.Exit(1)
	}
	if err := os.Rename(tmpName, zshrc); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to update .zshrc: %v\n", err)
		os.Exit(1)
	}
}
