package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/cyb33rr/rtlog/internal/state"
)

var newCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create and activate a new engagement",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		if err := logfile.ValidateEngagementName(name); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid engagement name: %v\n", err)
			os.Exit(1)
		}

		dir := logfile.LogDir()
		logPath := filepath.Join(dir, name+".jsonl")

		// Create log directory and file atomically
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating log directory: %v\n", err)
			os.Exit(1)
		}

		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			if os.IsExist(err) {
				fmt.Fprintf(os.Stderr, "Engagement '%s' already exists.\n", name)
			} else {
				fmt.Fprintf(os.Stderr, "Error creating log file: %v\n", err)
			}
			os.Exit(1)
		}
		f.Close()

		// Update state: set engagement, clear tag and note
		if _, err := state.UpdateState(map[string]string{
			state.KeyEngagement: name,
			state.KeyTag:        "",
			state.KeyNote:       "",
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("[rtlog] Engagement: %s -> %s\n", name, logPath)
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}
