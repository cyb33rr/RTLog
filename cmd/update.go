package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cyb33rr/rtlog/internal/update"
	"github.com/spf13/cobra"
)

var updateForce bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update rtlog to the latest version",
	Long: `Check GitHub Releases for the latest version and update the binary in-place.

If rtlog was installed via 'go install', prints the appropriate command instead.
Use --force to re-download even if the current version matches.`,
	Args: cobra.NoArgs,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&updateForce, "force", false, "re-download even if up to date")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	fmt.Printf("Current version: %s\n", Version)
	fmt.Println("Checking for updates...")

	rel, err := update.FetchLatestRelease(update.GitHubAPIURL)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	// Dev build confirmation
	if update.IsDevVersion(Version) {
		fmt.Printf("Current version is 'dev' (local build). Update will replace with %s.\n", rel.TagName)
		fmt.Print("Continue? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(answer)) != "y" {
			fmt.Println("Update cancelled.")
			return nil
		}
	} else if !updateForce && update.CompareVersions(Version, rel.TagName) >= 0 {
		fmt.Printf("Already up to date (%s).\n", Version)
		update.ClearUpdateAvailable()
		update.WriteLastCheck()
		return nil
	}

	// Resolve binary path
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine binary path: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return fmt.Errorf("cannot resolve binary path: %w", err)
	}

	// Check if installed via go install
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, _ := os.UserHomeDir()
		gopath = filepath.Join(home, "go")
	}
	gobin := os.Getenv("GOBIN")

	if update.IsGoInstalled(self, gopath, gobin) {
		fmt.Println("You installed rtlog via 'go install'. Run:")
		fmt.Println("  go install github.com/cyb33rr/rtlog@latest")
		return nil
	}

	// Find matching asset
	assetURL, err := update.FindAssetURL(rel.Assets, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	fmt.Printf("Downloading %s...\n", rel.TagName)

	// Download to temp file in same directory (for atomic rename)
	tmpPath := self + ".update"
	if err := update.DownloadBinary(assetURL, tmpPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Verify
	if err := update.VerifyBinary(tmpPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("downloaded file is invalid: %w", err)
	}

	// Replace
	if err := update.ReplaceBinary(tmpPath, self); err != nil {
		os.Remove(tmpPath)
		if os.IsPermission(err) {
			return fmt.Errorf("%w\nTry: sudo rtlog update", err)
		}
		return err
	}

	// Cleanup state
	update.ClearUpdateAvailable()
	update.WriteLastCheck()

	fmt.Printf("Updated rtlog: %s → %s\n", Version, rel.TagName)
	return nil
}
