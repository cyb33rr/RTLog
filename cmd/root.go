package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cyb33rr/rtlog/internal/db"
	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/extract"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/cyb33rr/rtlog/internal/update"
	"github.com/spf13/cobra"
)

// Version is set via ldflags at build time.
var Version = "dev"

// Engagement override from -e flag.
var engagementFlag string

var rootCmd = &cobra.Command{
	Use:   "rtlog",
	Short: "Query and analyze red team operation logs",
	Long: `Query and analyze red team operation logs from ~/.rt/logs/.

Log entries are stored in SQLite databases at ~/.rt/logs/<engagement>.db.
If no engagement is specified with -e, the most recently modified database is used.`,
	Version: Version,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&engagementFlag, "engagement", "e", "", "engagement name (defaults to most recent)")
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil // non-fatal
		}
		configPath := filepath.Join(home, ".rt", "extract.conf")

		// Auto-create from embedded default if missing
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			if data, e := embeddedFS.ReadFile("extract.conf"); e == nil {
				rtDir := filepath.Join(home, ".rt")
				_ = os.MkdirAll(rtDir, 0755)
				_ = os.WriteFile(configPath, data, 0644)
			}
		}

		// Load extraction config (primary source)
		_ = extract.LoadUserConfig(configPath)

		// Background version check: read state from *previous* run before
		// launching goroutine, so notification reflects prior state only.
		if shouldRunUpdateCheck(cmd) {
			pendingUpdate = update.ReadUpdateAvailable()
			if update.ShouldCheck() {
				go update.BackgroundCheck(Version, update.GitHubAPIURL)
			}
		}

		return nil
	}

	rootCmd.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
		if pendingUpdate != "" {
			fmt.Fprintf(os.Stderr, "\nUpdate available: %s (current: %s). Run 'rtlog update' to upgrade.\n", pendingUpdate, Version)
		}
		return nil
	}
}

// pendingUpdate holds the update-available version read from a *previous* run.
var pendingUpdate string

// shouldRunUpdateCheck returns true if the background update check should run.
func shouldRunUpdateCheck(cmd *cobra.Command) bool {
	if os.Getenv("RTLOG_NO_UPDATE_CHECK") == "1" {
		return false
	}
	if update.IsDevVersion(Version) {
		return false
	}
	if !display.IsTTY {
		return false
	}
	name := cmd.Name()
	if name == "log" || name == "update" {
		return false
	}
	return true
}

// openEngagementDB opens the SQLite database for the current engagement.
func openEngagementDB() (*db.DB, error) {
	logPath, err := logfile.GetLogPath(engagementFlag)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(logPath)
	eng := logfile.EngagementName(logPath)
	return db.Open(dir, eng)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
