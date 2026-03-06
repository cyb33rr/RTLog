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
		path := logfile.GetLogPath(engagementFlag)

		var dateFilter *time.Time
		if showToday {
			now := time.Now()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			dateFilter = &today
		} else if showDate != "" {
			d, err := time.Parse("2006-01-02", showDate)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid date format: %s (expected YYYY-MM-DD)\n", showDate)
				os.Exit(1)
			}
			dateFilter = &d
		}

		entries, err := logfile.LoadEntries(path, dateFilter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
			os.Exit(1)
		}

		if len(entries) == 0 {
			label := ""
			if dateFilter != nil {
				label = fmt.Sprintf(" for %s", dateFilter.Format("2006-01-02"))
			}
			fmt.Printf("No entries found%s in %s\n", label, logfile.EngagementName(path))
			return
		}

		header := fmt.Sprintf("--- %s ---", logfile.EngagementName(path))
		if dateFilter != nil {
			header += fmt.Sprintf("  [%s]", dateFilter.Format("2006-01-02"))
		}

		idxWidth := len(fmt.Sprintf("%d", len(entries)))

		// Convert to display.Entry maps
		entryMaps := make([]display.Entry, len(entries))
		for i, e := range entries {
			entryMaps[i] = logfile.ToMap(e)
		}

		if showOutput {
			fmt.Println(display.Colorize(header, display.Bold))
			fmt.Println()
			for i, m := range entryMaps {
				fmt.Println(display.FmtEntry(m, i+1, idxWidth, false))
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
				fmt.Println(display.FmtEntry(m, i+1, idxWidth))
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
