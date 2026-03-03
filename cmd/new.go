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

		dir := logfile.LogDir()
		logPath := filepath.Join(dir, name+".jsonl")

		// Check if engagement already exists
		if _, err := os.Stat(logPath); err == nil {
			fmt.Fprintf(os.Stderr, "Engagement '%s' already exists.\n", name)
			os.Exit(1)
		}

		// Update state: set engagement, clear tag and note
		if _, err := state.UpdateState(map[string]string{
			"engagement": name,
			"tag":        "",
			"note":       "",
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
			os.Exit(1)
		}

		// Create log directory and file
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating log directory: %v\n", err)
			os.Exit(1)
		}

		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating log file: %v\n", err)
			os.Exit(1)
		}
		f.Close()

		fmt.Printf("[rtlog] Engagement: %s -> %s\n", name, logPath)
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}
