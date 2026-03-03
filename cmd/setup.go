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
  3. Copy this binary to ~/.rt/rtlog
  4. Create symlink ~/.local/bin/rtlog -> ~/.rt/rtlog
  5. Configure ~/.zshrc (PATH + hook source line)`,
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

	// 1. Create directories
	setupCreateDir(logDir)
	setupCreateDir(localBin)

	// 2. Write embedded files
	setupWriteEmbedded("hook.zsh", filepath.Join(rtDir, "hook.zsh"))
	setupWriteEmbedded("tools.conf", filepath.Join(rtDir, "tools.conf"))

	// 3. Copy self to ~/.rt/rtlog
	setupCopySelf(filepath.Join(rtDir, "rtlog"))

	// 4. Symlink ~/.local/bin/rtlog -> ~/.rt/rtlog
	setupSymlink(filepath.Join(localBin, "rtlog"), filepath.Join(rtDir, "rtlog"))

	// 5. Configure ~/.zshrc
	setupZshrc(zshrc, localBin, rtDir)

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
func setupSymlink(link, target string) {
	updated := false
	if existing, err := os.Readlink(link); err == nil {
		if existing == target {
			fmt.Printf("[ok] Symlink exists: %s -> %s\n", link, target)
			return
		}
		os.Remove(link)
		updated = true
	} else if _, err := os.Stat(link); err == nil {
		fmt.Printf("[!]  %s exists but is not a symlink, skipping\n", link)
		return
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
}

// setupZshrc ensures PATH and hook source lines are in .zshrc.
func setupZshrc(zshrc, localBin, rtDir string) {
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
	removedOld := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Remove old Python/repo hook source lines (not our .rt/ one)
		if strings.Contains(trimmed, "source") && strings.Contains(trimmed, "hook.zsh") &&
			!strings.Contains(trimmed, ".rt/hook.zsh") {
			removedOld = true
			continue
		}

		// Remove "# Red Team Operation Logger" comment only if followed by an old hook line
		if trimmed == "# Red Team Operation Logger" {
			nextIsOldHook := false
			if i+1 < len(lines) {
				next := strings.TrimSpace(lines[i+1])
				nextIsOldHook = strings.Contains(next, "source") &&
					strings.Contains(next, "hook.zsh") &&
					!strings.Contains(next, ".rt/hook.zsh")
			}
			if nextIsOldHook {
				removedOld = true
				continue
			}
		}

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

	if removedOld {
		fmt.Println("[+]  Removed old hook source line(s) from .zshrc")
	}

	// Append PATH export if missing
	if !hasPathExport {
		newLines = append(newLines, "", `export PATH="$HOME/.local/bin:$PATH"`)
		fmt.Printf("[+]  Added %s to PATH in .zshrc\n", localBin)
	} else {
		fmt.Printf("[ok] %s already in PATH\n", localBin)
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
