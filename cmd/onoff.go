package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/state"
)

var onCmd = &cobra.Command{
	Use:   "on",
	Short: "Enable logging",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if _, err := state.UpdateState(map[string]string{"enabled": "1"}); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[rtlog] Logging enabled")
	},
}

var offCmd = &cobra.Command{
	Use:   "off",
	Short: "Disable logging",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if _, err := state.UpdateState(map[string]string{"enabled": "0"}); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[rtlog] Logging disabled")
	},
}

func init() {
	rootCmd.AddCommand(onCmd)
	rootCmd.AddCommand(offCmd)
}
