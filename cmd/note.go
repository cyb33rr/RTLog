package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/state"
)

var noteCmd = &cobra.Command{
	Use:   "note <text...>",
	Short: "Queue a one-shot note for the next command",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		text := strings.Join(args, " ")
		if _, err := state.UpdateState(map[string]string{state.KeyNote: text}); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[rtlog] Note queued for next command")
	},
}

func init() {
	rootCmd.AddCommand(noteCmd)
}
