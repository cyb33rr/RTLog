package cmd

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/filter"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/spf13/cobra"
)


var showToday bool
var showDate string
var showOutput bool
var showRegex string

var showCmd = &cobra.Command{
	Use:   "show [keyword]",
	Short: "Pretty-print log entries",
	Long:  "Display log entries in a human-readable format, optionally filtered by date.\nWith a keyword argument, performs a non-interactive search with highlighting.",
	Args:  cobra.MaximumNArgs(1),
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

		engName := logfile.EngagementName(path)
		keyword := ""
		if len(args) > 0 {
			keyword = args[0]
		}

		if keyword != "" && showRegex != "" {
			fmt.Fprintf(os.Stderr, "Cannot use keyword argument with -r flag\n")
			os.Exit(1)
		}

		// --- Keyword search branch ---
		if keyword != "" {
			var dateLabel string
			var matches []logfile.LogEntry
			if showDate != "" {
				matches, err = d.SearchByDate(keyword, showDate)
				dateLabel = showDate
			} else if showToday {
				today := time.Now().UTC().Format("2006-01-02")
				matches, err = d.SearchByDate(keyword, today)
				dateLabel = today
			} else {
				matches, err = d.Search(keyword)
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error searching entries: %v\n", err)
				os.Exit(1)
			}

			if len(matches) == 0 {
				label := ""
				if dateLabel != "" {
					label = fmt.Sprintf(" for %s", dateLabel)
				}
				fmt.Printf("No matches for '%s'%s in %s\n", keyword, label, engName)
				return
			}

			header := fmt.Sprintf("--- %d match(es) for '%s' in %s ---", len(matches), keyword, engName)
			if dateLabel != "" {
				header += fmt.Sprintf("  [%s]", dateLabel)
			}
			fmt.Println(display.Colorize(header, display.Bold))
			fmt.Println()

			pattern := regexp.MustCompile("(?i)" + regexp.QuoteMeta(keyword))
			idxWidth := len(fmt.Sprintf("%d", len(matches)))
			for i, entry := range matches {
				m := logfile.ToMap(entry)
				if showOutput {
					fmt.Println(display.FmtEntryHighlight(m, pattern, i+1, idxWidth, false))
					display.PrintOutputBlock(m, true)
				} else {
					fmt.Println(display.FmtEntryHighlight(m, pattern, i+1, idxWidth))
				}
			}

			fmt.Println()
			fmt.Printf("%s\n", display.Colorize(
				fmt.Sprintf("%d result(s)", len(matches)),
				display.Dim,
			))
			return
		}

		// --- Regex search branch ---
		if showRegex != "" {
			re, err := regexp.Compile(showRegex)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid regex: %v\n", err)
				os.Exit(1)
			}

			var dateLabel string
			var allEntries []logfile.LogEntry
			if showDate != "" {
				allEntries, err = d.LoadByDate(showDate)
				dateLabel = showDate
			} else if showToday {
				today := time.Now().UTC().Format("2006-01-02")
				allEntries, err = d.LoadByDate(today)
				dateLabel = today
			} else {
				allEntries, err = d.LoadAll()
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
				os.Exit(1)
			}

			matches, err := filter.MatchRegex(allEntries, showRegex)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error applying regex: %v\n", err)
				os.Exit(1)
			}

			if len(matches) == 0 {
				label := ""
				if dateLabel != "" {
					label = fmt.Sprintf(" for %s", dateLabel)
				}
				fmt.Printf("No matches for regex '%s'%s in %s\n", showRegex, label, engName)
				return
			}

			header := fmt.Sprintf("--- %d match(es) for regex '%s' in %s ---", len(matches), showRegex, engName)
			if dateLabel != "" {
				header += fmt.Sprintf("  [%s]", dateLabel)
			}
			fmt.Println(display.Colorize(header, display.Bold))
			fmt.Println()

			idxWidth := len(fmt.Sprintf("%d", len(matches)))
			for i, entry := range matches {
				m := logfile.ToMap(entry)
				if showOutput {
					fmt.Println(display.FmtEntryHighlight(m, re, i+1, idxWidth, false))
					display.PrintOutputBlock(m, true)
				} else {
					fmt.Println(display.FmtEntryHighlight(m, re, i+1, idxWidth))
				}
			}

			fmt.Println()
			fmt.Printf("%s\n", display.Colorize(
				fmt.Sprintf("%d result(s)", len(matches)),
				display.Dim,
			))
			return
		}

		// --- No keyword: existing show behavior ---
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
			fmt.Printf("No entries found%s in %s\n", label, engName)
			return
		}

		// Convert to display.Entry maps
		entryMaps := make([]display.Entry, len(entries))
		for i, e := range entries {
			entryMaps[i] = logfile.ToMap(e)
		}

		if showOutput {
			// Non-interactive --all: reverse for newest-first, use FmtEntry
			for i, j := 0, len(entryMaps)-1; i < j; i, j = i+1, j-1 {
				entryMaps[i], entryMaps[j] = entryMaps[j], entryMaps[i]
			}
			header := fmt.Sprintf("--- %s ---", engName)
			if dateLabel != "" {
				header += fmt.Sprintf("  [%s]", dateLabel)
			}
			idxWidth := len(fmt.Sprintf("%d", len(entries)))
			n := len(entryMaps)
			origIdx := func(i int) int { return n - i }
			fmt.Println(display.Colorize(header, display.Bold))
			fmt.Println()
			for i, m := range entryMaps {
				fmt.Println(display.FmtEntry(m, origIdx(i), idxWidth, false))
				display.PrintOutputBlock(m, true)
			}
		} else if display.IsTTY {
			// Interactive TUI: entries in chronological order (oldest first)
			sel := display.NewSelector(entryMaps)
			if err := sel.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Non-interactive pipe: reverse for newest-first, use FmtEntry
			for i, j := 0, len(entryMaps)-1; i < j; i, j = i+1, j-1 {
				entryMaps[i], entryMaps[j] = entryMaps[j], entryMaps[i]
			}
			header := fmt.Sprintf("--- %s ---", engName)
			if dateLabel != "" {
				header += fmt.Sprintf("  [%s]", dateLabel)
			}
			idxWidth := len(fmt.Sprintf("%d", len(entries)))
			n := len(entryMaps)
			origIdx := func(i int) int { return n - i }
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
	showCmd.Flags().StringVarP(&showRegex, "regex", "r", "", "search by regex pattern across cmd, tool, cwd, tag, note")
	showCmd.MarkFlagsMutuallyExclusive("today", "date")
	rootCmd.AddCommand(showCmd)
}
