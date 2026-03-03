package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/state"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show full state overview",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		st := state.ReadState()

		eng := st[state.KeyEngagement]
		if eng == "" {
			eng = "(none)"
		}
		tag := st[state.KeyTag]
		if tag == "" {
			tag = "(none)"
		}
		note := st[state.KeyNote]
		if note == "" {
			note = "(none)"
		}
		enabled := "on"
		if st[state.KeyEnabled] != "1" {
			enabled = "off"
		}
		capture := "on"
		if st[state.KeyCapture] != "1" {
			capture = "off"
		}

		enabledColor := display.Green
		if enabled == "off" {
			enabledColor = display.Red
		}
		captureColor := display.Green
		if capture == "off" {
			captureColor = display.Red
		}

		fmt.Println(display.Colorize("--- rtlog status ---", display.Bold))
		fmt.Printf("  Engagement:  %s\n", display.Colorize(eng, display.Cyan))
		fmt.Printf("  Tag:         %s\n", display.Colorize(tag, display.Yellow))
		fmt.Printf("  Note:        %s\n", note)
		fmt.Printf("  Logging:     %s\n", display.Colorize(enabled, enabledColor))
		fmt.Printf("  Capture:     %s\n", display.Colorize(capture, captureColor))
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
