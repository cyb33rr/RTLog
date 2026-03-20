package cmd

import (
	"os"
	"path/filepath"
	"strings"
)

// isGoInstall reports whether the running binary was installed via 'go install'.
// It resolves the executable path and the Go bin directory through symlinks and
// returns true if the executable lives inside the Go bin directory.
// On any error, it returns false (safe default — treats as manual install).
func isGoInstall() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return false
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	goBinDir, _ := resolveGoBinDir(home, os.Getenv("GOPATH"), os.Getenv("GOBIN"))

	goBinDir, err = filepath.EvalSymlinks(goBinDir)
	if err != nil {
		return false
	}

	return strings.HasPrefix(exe, goBinDir+string(filepath.Separator))
}
