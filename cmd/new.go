package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/db"
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
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		dbPath := filepath.Join(dir, name+".db")
		if _, err := os.Stat(dbPath); err == nil {
			fmt.Fprintf(os.Stderr, "error: engagement %q already exists\n", name)
			os.Exit(1)
		}

		d, err := db.Open(dir, name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		d.Close()

		// Update state: set engagement, clear tag and note
		if _, err := state.UpdateState(map[string]string{
			state.KeyEngagement: name,
			state.KeyTag:        "",
			state.KeyNote:       "",
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("[rtlog] Engagement: %s -> %s\n", name, dbPath)
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}
