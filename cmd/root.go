package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cyb33rr/rtlog/internal/extract"
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

Log files are stored as JSONL in ~/.rt/logs/<engagement>.jsonl.
If no engagement is specified with -e, the most recently modified file is used.`,
	Version: Version,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&engagementFlag, "engagement", "e", "", "engagement name (defaults to most recent)")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil // non-fatal
		}
		configPath := filepath.Join(home, ".rt", "extract.conf")
		_ = extract.LoadUserConfig(configPath)
		return nil
	}
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
