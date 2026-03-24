package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/state"
)

var outputFlag bool

var onCmd = &cobra.Command{
	Use:   "on",
	Short: "Enable logging and output capture (--output for capture only)",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if outputFlag {
			if _, err := state.UpdateState(map[string]string{state.KeyCapture: "1"}); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("[rtlog] Output capture enabled")
			return
		}
		if _, err := state.UpdateState(map[string]string{state.KeyEnabled: "1", state.KeyCapture: "1"}); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[rtlog] Logging enabled (with output capture)")
	},
}

var offCmd = &cobra.Command{
	Use:   "off",
	Short: "Disable logging and output capture (--output for capture only)",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if outputFlag {
			if _, err := state.UpdateState(map[string]string{state.KeyCapture: "0"}); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("[rtlog] Output capture disabled")
			return
		}
		if _, err := state.UpdateState(map[string]string{state.KeyEnabled: "0", state.KeyCapture: "0"}); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[rtlog] Logging disabled (including output capture)")
	},
}

func init() {
	onCmd.Flags().BoolVar(&outputFlag, "output", false, "toggle output capture instead of logging")
	offCmd.Flags().BoolVar(&outputFlag, "output", false, "toggle output capture instead of logging")
	rootCmd.AddCommand(onCmd)
	rootCmd.AddCommand(offCmd)
}
