package cmd

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"os/exec"
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
// Preserved: logs/, state, tools.conf, extract.conf, rtlog binary.
func setupCleanup(rtDir string) {
	denylist := []string{
		"hook.zsh",
		"hook.bash",
		"hook-noninteractive.zsh",
		"hook-noninteractive.bash",
		"bash-preexec.sh",
		"last-update-check",
		"update-available",
	}
	for _, name := range denylist {
		path := filepath.Join(rtDir, name)
		if err := os.Remove(path); err == nil {
			fmt.Printf("[~]  Cleaned up: %s\n", name)
		}
	}
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

// setupCopySelfTo copies the running binary to dst using atomic temp+rename.
// Returns an error instead of exiting, so callers can handle permission errors.
func setupCopySelfTo(dst string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine own path: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return fmt.Errorf("cannot resolve own path: %w", err)
	}

	// If we're already at the destination, skip
	absDst, _ := filepath.Abs(dst)
	if self == absDst {
		fmt.Printf("[ok] Binary already at %s\n", dst)
		return nil
	}

	// Check if existing binary is identical
	selfInfo, err := os.Stat(self)
	if err == nil {
		dstInfo, derr := os.Stat(dst)
		if derr == nil && selfInfo.Size() == dstInfo.Size() {
			selfData, e1 := os.ReadFile(self)
			dstData, e2 := os.ReadFile(dst)
			if e1 == nil && e2 == nil && bytes.Equal(selfData, dstData) {
				fmt.Printf("[ok] Binary is up to date: %s\n", dst)
				return nil
			}
		}
	}

	// Atomic copy: write to temp + rename
	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, ".rtlog.")
	if err != nil {
		if os.IsPermission(err) {
			return err
		}
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	src, err := os.Open(self)
	if err != nil {
		tmp.Close()
		return fmt.Errorf("cannot open self: %w", err)
	}
	defer src.Close()

	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to copy binary: %w", err)
	}
	if err := tmp.Chmod(0755); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to set permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		if os.IsPermission(err) {
			return err
		}
		return fmt.Errorf("failed to install binary: %w", err)
	}
	fmt.Printf("[+]  Installed binary: %s\n", dst)
	return nil
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

// installKind classifies how rtlog is installed.
type installKind int

const (
	installFresh     installKind = iota // not found on PATH
	installDefault                      // found at ~/.rt/rtlog (directly or via symlink)
	installCustom                       // found on PATH at a non-default location
	installGoInstall                    // found inside GOPATH/bin or GOBIN
)

// detectBinaryPath finds rtlog on PATH and classifies the install type.
// Returns the kind and the resolved (real) path to the binary.
// For installFresh, the path is empty.
func detectBinaryPath(home string) (installKind, string) {
	found, err := exec.LookPath("rtlog")
	if err != nil {
		return installFresh, ""
	}

	resolved, err := filepath.EvalSymlinks(found)
	if err != nil {
		return installFresh, ""
	}

	// Check go install first (highest priority)
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(home, "go")
	}
	gobin := os.Getenv("GOBIN")
	if isGoInstalled(resolved, gopath, gobin) {
		return installGoInstall, resolved
	}

	// Check if it resolves to the default ~/.rt/rtlog
	defaultBin := filepath.Join(home, ".rt", "rtlog")
	if resolved == defaultBin {
		return installDefault, resolved
	}

	return installCustom, resolved
}

// setupShellRc ensures PATH and hook source lines are in the given rc file.
// hookFile is "hook.zsh" or "hook.bash". rcName is ".zshrc" or ".bashrc" (for messages).
func setupShellRc(rcFile, localBin, rtDir string, addPathExport bool, goBinExportLine, hookFile, rcName string) error {
	sourceLine := fmt.Sprintf("source %s/.rt/%s", "$HOME", hookFile)

	// Read existing content
	content, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot read %s: %w", rcFile, err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	hasPathExport := false
	hasSourceLine := false
	hasGoBinExport := false

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

		// Check for existing Go bin PATH export
		if goBinExportLine != "" && !strings.HasPrefix(trimmed, "#") && trimmed == goBinExportLine {
			hasGoBinExport = true
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
	} else if goBinExportLine == "" {
		fmt.Printf("[ok] Binary on PATH, skipping PATH export in %s\n", rcName)
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
	// Preserve original permissions
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

// isGoInstalled checks if the binary path is inside GOPATH/bin or GOBIN.
func isGoInstalled(binPath, gopath, gobin string) bool {
	if gobin != "" && filepath.Dir(binPath) == gobin {
		return true
	}
	if gopath != "" {
		if filepath.Dir(binPath) == filepath.Join(gopath, "bin") {
			return true
		}
	}
	return false
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
	exportLine = fmt.Sprintf(`export PATH="%s:$PATH"`, pathStr)
	return dir, exportLine
}
