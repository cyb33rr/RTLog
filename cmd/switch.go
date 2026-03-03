package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/cyb33rr/rtlog/internal/state"
)

var switchCmd = &cobra.Command{
	Use:   "switch <name>",
	Short: "Switch to an existing engagement",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		dir := logfile.LogDir()
		logPath := filepath.Join(dir, name+".jsonl")

		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Engagement '%s' not found.\n", name)
			files := logfile.AvailableEngagements()
			if len(files) > 0 {
				names := make([]string, len(files))
				for i, f := range files {
					names[i] = logfile.EngagementName(f)
				}
				fmt.Fprintf(os.Stderr, "Available: %s\n", joinComma(names))
			}
			os.Exit(1)
		}

		if _, err := state.UpdateState(map[string]string{
			"engagement": name,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("[rtlog] Switched to: %s\n", name)
	},
}

func joinComma(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

func init() {
	rootCmd.AddCommand(switchCmd)
}
