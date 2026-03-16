package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"golang.org/x/term"
)

var uninstallYes bool

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove rtlog from the system",
	Long: `Remove rtlog installation artifacts:

  1. Remove symlink ~/.local/bin/rtlog (if present)
  2. Remove hook and PATH export lines from ~/.zshrc and ~/.bashrc
  3. Optionally delete ~/.rt/ (prompts unless -y)
  4. If installed via go install, advises how to remove the binary`,
	Args: cobra.NoArgs,
	Run:  runUninstall,
}

func init() {
	uninstallCmd.Flags().BoolVarP(&uninstallYes, "yes", "y", false, "skip confirmation prompts")
	rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(cmd *cobra.Command, args []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine home directory: %v\n", err)
		os.Exit(1)
	}

	rtDir := filepath.Join(home, ".rt")
	localBin := filepath.Join(home, ".local", "bin")
	zshrc := filepath.Join(home, ".zshrc")
	bashrc := filepath.Join(home, ".bashrc")
	symlinkPath := filepath.Join(localBin, "rtlog")
	binaryPath := filepath.Join(rtDir, "rtlog")

	fmt.Println("=== Red Team Operation Logger - Uninstaller ===")
	fmt.Println()

	// 1. Remove symlink
	uninstallRemoveSymlink(symlinkPath, binaryPath)

	// 2. Remove hook lines from shell rc files
	uninstallCleanShellRc(zshrc, ".rt/hook.zsh", ".zshrc")
	uninstallCleanShellRc(bashrc, ".rt/hook.bash", ".bashrc")

	// Clean non-interactive hook lines from .zshenv
	zshenv := filepath.Join(home, ".zshenv")
	uninstallCleanNonInteractive(zshenv, ".zshenv")

	// Clean BASH_ENV export from rc files
	uninstallCleanNonInteractive(zshrc, ".zshrc")
	uninstallCleanNonInteractive(bashrc, ".bashrc")

	// 3. Remove ~/.rt/ (prompt first)
	uninstallRemoveDir(rtDir)

	// 4. Advise on go install binary if applicable
	uninstallAdviseGoInstall()

	fmt.Println()
	fmt.Println("=== Uninstall complete ===")
	fmt.Println()
	fmt.Println("Open a new shell to apply changes.")
}

// uninstallRemoveSymlink removes the rtlog symlink if it points to our binary.
func uninstallRemoveSymlink(link, expectedTarget string) {
	target, err := os.Readlink(link)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("[ok] No symlink at %s\n", link)
			return
		}
		// Exists but not a symlink
		fmt.Printf("[!]  %s exists but is not a symlink, skipping\n", link)
		return
	}

	if target != expectedTarget {
		fmt.Printf("[!]  %s points to %s (not ours), skipping\n", link, target)
		return
	}

	if err := os.Remove(link); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to remove symlink: %v\n", err)
		return
	}
	fmt.Printf("[-]  Removed symlink: %s\n", link)
}

// uninstallCleanShellRc removes hook-related lines from a shell rc file.
// hookPattern is the hook path to match (e.g. ".rt/hook.zsh" or ".rt/hook.bash").
// rcName is a display name (e.g. ".zshrc" or ".bashrc").
func uninstallCleanShellRc(rcFile, hookPattern, rcName string) {
	content, err := os.ReadFile(rcFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("[ok] No %s found\n", rcName)
			return
		}
		fmt.Fprintf(os.Stderr, "[!]  Cannot read %s: %v\n", rcFile, err)
		return
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	removed := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Remove "# Red Team Operation Logger" comment
		if trimmed == "# Red Team Operation Logger" {
			removed = true
			continue
		}

		// Remove our hook source line (match the specific hook pattern)
		if strings.Contains(trimmed, "source") && strings.Contains(trimmed, hookPattern) {
			removed = true
			continue
		}

		// Remove PATH export added by setup
		if trimmed == `export PATH="$HOME/.local/bin:$PATH"` {
			removed = true
			continue
		}

		newLines = append(newLines, line)
	}

	if !removed {
		fmt.Printf("[ok] No hook lines in %s\n", rcName)
		return
	}

	// Collapse consecutive blank lines left by removal
	newLines = collapseBlankLines(newLines)

	// Atomic write
	newContent := strings.Join(newLines, "\n")
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
		fmt.Fprintf(os.Stderr, "[!]  Failed to write %s: %v\n", rcName, err)
		return
	}
	if info, err := os.Stat(rcFile); err == nil {
		tmp.Chmod(info.Mode())
	} else {
		tmp.Chmod(0644)
	}
	if err := tmp.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to close temp file: %v\n", err)
		return
	}
	if err := os.Rename(tmpName, rcFile); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to update %s: %v\n", rcName, err)
		return
	}
	fmt.Printf("[-]  Removed hook lines from %s\n", rcName)
}

// uninstallCleanNonInteractive removes non-interactive hook lines from a file:
// - "# RTLog non-interactive capture" comment
// - "# RTLog non-interactive bash capture" comment
// - source line for hook-noninteractive.zsh
// - BASH_ENV export for hook-noninteractive.bash
func uninstallCleanNonInteractive(rcFile, rcName string) {
	content, err := os.ReadFile(rcFile)
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	removed := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "# RTLog non-interactive capture" ||
			trimmed == "# RTLog non-interactive bash capture" ||
			(strings.Contains(trimmed, "source") && strings.Contains(trimmed, "hook-noninteractive.zsh")) ||
			(strings.Contains(trimmed, "BASH_ENV") && strings.Contains(trimmed, "hook-noninteractive.bash")) {
			removed = true
			continue
		}
		newLines = append(newLines, line)
	}

	if !removed {
		return
	}

	newLines = collapseBlankLines(newLines)
	newContent := strings.Join(newLines, "\n")
	dir := filepath.Dir(rcFile)
	tmp, err := os.CreateTemp(dir, "."+rcName+".")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	tmp.WriteString(newContent)
	if info, err := os.Stat(rcFile); err == nil {
		tmp.Chmod(info.Mode())
	} else {
		tmp.Chmod(0644)
	}
	tmp.Close()
	os.Rename(tmpName, rcFile)
	fmt.Printf("[-]  Removed non-interactive hook lines from %s\n", rcName)
}

// uninstallRemoveDir removes ~/.rt/ after confirmation.
func uninstallRemoveDir(rtDir string) {
	if _, err := os.Stat(rtDir); os.IsNotExist(err) {
		fmt.Printf("[ok] No %s directory\n", rtDir)
		return
	}

	if !uninstallYes {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Printf("[ok] Kept %s (non-interactive, use -y to force)\n", rtDir)
			return
		}
		fmt.Printf("Delete %s? This contains runtime files and all engagement logs. [y/N] ", rtDir)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(answer)) != "y" {
			fmt.Printf("[ok] Kept %s\n", rtDir)
			return
		}
	}

	if err := os.RemoveAll(rtDir); err != nil {
		fmt.Fprintf(os.Stderr, "[!]  Failed to remove %s: %v\n", rtDir, err)
		return
	}
	fmt.Printf("[-]  Removed %s\n", rtDir)
}

// uninstallAdviseGoInstall checks if the running binary lives in GOBIN/GOPATH
// and advises the user to remove it manually.
func uninstallAdviseGoInstall() {
	self, err := os.Executable()
	if err != nil {
		return
	}
	self, _ = filepath.EvalSymlinks(self)
	selfDir := filepath.Dir(self)

	// Check against GOBIN or GOPATH/bin or ~/go/bin
	gobin := os.Getenv("GOBIN")
	if gobin != "" {
		gobin, _ = filepath.Abs(gobin)
	}
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			gopath = filepath.Join(home, "go")
		}
	}
	gopathBin := ""
	if gopath != "" {
		gopathBin, _ = filepath.Abs(filepath.Join(gopath, "bin"))
	}

	if (gobin != "" && selfDir == gobin) || (gopathBin != "" && selfDir == gopathBin) {
		fmt.Printf("[!]  Binary at %s was installed via 'go install'\n", self)
		fmt.Println("     Remove it with: rm", self)
	}
}
