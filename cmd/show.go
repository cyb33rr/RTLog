package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/spf13/cobra"
)


var showToday bool
var showDate string
var showOutput bool

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Pretty-print log entries",
	Long:  "Display log entries in a human-readable format, optionally filtered by date.",
	Run: func(cmd *cobra.Command, args []string) {
		// Validate date flag early
		if showDate != "" {
			if _, err := time.Parse("2006-01-02", showDate); err != nil {
				fmt.Fprintf(os.Stderr, "Invalid date format: %s (expected YYYY-MM-DD)\n", showDate)
				os.Exit(1)
			}
		}

		path, err := logfile.GetLogPath(engagementFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		d, err := openEngagementDB()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer d.Close()
		var dateLabel string
		var entries []logfile.LogEntry
		if showDate != "" {
			entries, err = d.LoadByDate(showDate)
			dateLabel = showDate
		} else if showToday {
			today := time.Now().UTC().Format("2006-01-02")
			entries, err = d.LoadByDate(today)
			dateLabel = today
		} else {
			entries, err = d.LoadAll()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
			os.Exit(1)
		}

		if len(entries) == 0 {
			label := ""
			if dateLabel != "" {
				label = fmt.Sprintf(" for %s", dateLabel)
			}
			fmt.Printf("No entries found%s in %s\n", label, logfile.EngagementName(path))
			return
		}

		header := fmt.Sprintf("--- %s ---", logfile.EngagementName(path))
		if dateLabel != "" {
			header += fmt.Sprintf("  [%s]", dateLabel)
		}

		idxWidth := len(fmt.Sprintf("%d", len(entries)))

		// Convert to display.Entry maps
		entryMaps := make([]display.Entry, len(entries))
		for i, e := range entries {
			entryMaps[i] = logfile.ToMap(e)
		}

		// Default to reverse order (newest first)
		for i, j := 0, len(entryMaps)-1; i < j; i, j = i+1, j-1 {
			entryMaps[i], entryMaps[j] = entryMaps[j], entryMaps[i]
		}

		n := len(entryMaps)
		origIdx := func(i int) int { return n - i }

		if showOutput {
			fmt.Println(display.Colorize(header, display.Bold))
			fmt.Println()
			for i, m := range entryMaps {
				fmt.Println(display.FmtEntry(m, origIdx(i), idxWidth, false))
				display.PrintOutputBlock(m, true)
			}
		} else if display.IsTTY {
			sel := display.NewSelector(entryMaps, header, idxWidth)
			if err := sel.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Println(display.Colorize(header, display.Bold))
			fmt.Println()
			for i, m := range entryMaps {
				fmt.Println(display.FmtEntry(m, origIdx(i), idxWidth))
			}
		}
	},
}

func init() {
	showCmd.Flags().BoolVar(&showToday, "today", false, "show only today's entries")
	showCmd.Flags().StringVar(&showDate, "date", "", "show entries for a specific date (YYYY-MM-DD)")
	showCmd.Flags().BoolVarP(&showOutput, "all", "a", false, "print all entries with their output (non-interactive)")
	showCmd.MarkFlagsMutuallyExclusive("today", "date")
	rootCmd.AddCommand(showCmd)
}
