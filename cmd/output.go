package cmd

import (
	"fmt"
	"os"

	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/spf13/cobra"
)

var outputRaw bool

var outputCmd = &cobra.Command{
	Use:   "output <index>",
	Short: "View captured output for a specific command",
	Long:  "Display the captured stdout/stderr for a command by its index (1-based).",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var idx int
		if _, err := fmt.Sscanf(args[0], "%d", &idx); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid index: %s\n", args[0])
			os.Exit(1)
		}

		path := logfile.GetLogPath(engagementFlag)
		entries, err := logfile.LoadEntries(path, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
			os.Exit(1)
		}

		if len(entries) == 0 {
			fmt.Printf("No entries in %s\n", logfile.EngagementName(path))
			return
		}

		if idx < 1 || idx > len(entries) {
			fmt.Fprintf(os.Stderr, "Index out of range (1-%d)\n", len(entries))
			os.Exit(1)
		}

		entry := entries[idx-1]
		m := logfile.ToMap(entry)
		idxWidth := len(fmt.Sprintf("%d", len(entries)))
		fmt.Println(display.FmtEntry(m, idx, idxWidth))

		text := entry.Out
		if text == "" || len(trimSpace(text)) == 0 {
			fmt.Println(display.Colorize("  (no captured output)", display.Dim))
			return
		}

		if !outputRaw {
			text = display.RE_ANSI.ReplaceAllString(text, "")
		}

		fmt.Println()
		fmt.Print(text)
		if len(text) > 0 && text[len(text)-1] != '\n' {
			fmt.Println()
		}
	},
}

func trimSpace(s string) string {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return s
		}
	}
	return ""
}

func init() {
	outputCmd.Flags().BoolVar(&outputRaw, "raw", false, "show raw output including ANSI escape codes")
	rootCmd.AddCommand(outputCmd)
}
