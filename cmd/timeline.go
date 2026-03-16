package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/cyb33rr/rtlog/internal/timeutil"
	"github.com/spf13/cobra"
)

var timelineCmd = &cobra.Command{
	Use:   "timeline",
	Short: "Chronological view grouped by date and tag",
	Long:  "Group log entries by date and tag, showing a chronological operation flow.",
	Run: func(cmd *cobra.Command, args []string) {
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

		entries, err := d.LoadAll()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
			os.Exit(1)
		}

		if len(entries) == 0 {
			fmt.Printf("No entries in %s\n", logfile.EngagementName(path))
			return
		}

		// Group by date
		type dateGroup struct {
			date    string
			entries []logfile.LogEntry
		}
		dateOrder := []string{}
		byDate := map[string][]logfile.LogEntry{}
		var unknowns []logfile.LogEntry

		for _, entry := range entries {
			dateKey := parseEntryDate(entry.Ts)
			if dateKey == "" {
				unknowns = append(unknowns, entry)
				continue
			}
			if _, exists := byDate[dateKey]; !exists {
				dateOrder = append(dateOrder, dateKey)
			}
			byDate[dateKey] = append(byDate[dateKey], entry)
		}

		// Sort dates
		sort.Strings(dateOrder)

		for _, dateKey := range dateOrder {
			fmt.Println(display.Colorize(fmt.Sprintf("\n=== %s ===", dateKey), display.Bold))

			dayEntries := byDate[dateKey]

			// Group by tag within this date, preserving order
			tagOrder := []string{}
			byTag := map[string][]logfile.LogEntry{}
			for _, entry := range dayEntries {
				tag := entry.Tag
				if tag == "" {
					tag = "untagged"
				}
				if _, exists := byTag[tag]; !exists {
					tagOrder = append(tagOrder, tag)
				}
				byTag[tag] = append(byTag[tag], entry)
			}

			for _, tag := range tagOrder {
				fmt.Println(display.Colorize(fmt.Sprintf("  [%s]", tag), display.Yellow))
				for _, entry := range byTag[tag] {
					tsStr := formatTimeOnly(entry.Ts)
					cmdStr := strings.ReplaceAll(entry.Cmd, "\n", " ")
					dur := entry.Dur
					exitCode := entry.Exit
					exitColor := display.Green
					if exitCode != 0 {
						exitColor = display.Red
					}
					fmt.Printf("    %s  %s  (%s, %s)\n",
						tsStr, cmdStr,
						display.Colorize(fmt.Sprintf("%gs", dur), display.Dim),
						display.Colorize(fmt.Sprintf("exit:%d", exitCode), exitColor),
					)
				}
			}
		}

		// Handle unknowns
		if len(unknowns) > 0 {
			fmt.Println(display.Colorize("\n=== unknown date ===", display.Bold))
			idxWidth := len(fmt.Sprintf("%d", len(unknowns)))
			for i, entry := range unknowns {
				m := logfile.ToMap(entry)
				fmt.Println(display.FmtEntry(m, i+1, idxWidth))
			}
		}
	},
}

// parseEntryDate extracts YYYY-MM-DD from an ISO timestamp.
func parseEntryDate(ts string) string {
	if ts == "" {
		return ""
	}
	if t, err := timeutil.Parse(ts); err == nil {
		return t.Format("2006-01-02")
	}
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ""
}

// formatTimeOnly extracts HH:MM:SS from an ISO timestamp.
func formatTimeOnly(ts string) string {
	if ts == "" {
		return ""
	}
	if t, err := timeutil.Parse(ts); err == nil {
		return t.Format("15:04:05")
	}
	if len(ts) >= 8 {
		return ts[:8]
	}
	return ts
}

func init() {
	rootCmd.AddCommand(timelineCmd)
}
