package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/state"
)

var captureCmd = &cobra.Command{
	Use:   "capture [on|off]",
	Short: "Show or toggle output capture",
	Args:  cobra.MaximumNArgs(1),
	ValidArgs: []string{"on", "off"},
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			toggle := args[0]
			if toggle != "on" && toggle != "off" {
				fmt.Fprintf(os.Stderr, "Invalid argument: %s (must be 'on' or 'off')\n", toggle)
				os.Exit(1)
			}
			val := "1"
			if toggle == "off" {
				val = "0"
			}
			if _, err := state.UpdateState(map[string]string{"capture": val}); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
				os.Exit(1)
			}
			label := "enabled"
			if val == "0" {
				label = "disabled"
			}
			fmt.Printf("[rtlog] Output capture %s\n", label)
			return
		}

		// No args: show current state
		st := state.ReadState()
		val := st["capture"]
		label := "on"
		if val != "1" {
			label = "off"
		}
		fmt.Printf("[rtlog] Output capture: %s\n", label)
	},
}

func init() {
	rootCmd.AddCommand(captureCmd)
}
