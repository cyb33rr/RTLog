package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/cyb33rr/rtlog/internal/state"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all engagements (* marks active)",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		files := logfile.AvailableEngagements()
		if len(files) == 0 {
			fmt.Println("No engagements found.")
			return
		}

		st := state.ReadState()
		active := st[state.KeyEngagement]

		fmt.Println(display.Colorize("--- Engagements ---", display.Bold))
		for _, f := range files {
			name := logfile.EngagementName(f)
			count := logfile.CountEntries(f)
			marker := ""
			if name == active {
				marker = display.Colorize(" *", display.Green)
			}
			fmt.Printf("  %s%s  (%d entries)\n", name, marker, count)
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
