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
	"golang.org/x/term"
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

		termWidth := 80
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
			termWidth = w
		}

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
				fmt.Println(display.Colorize(fmt.Sprintf("    [%s]", tag), display.Yellow))
				for _, entry := range byTag[tag] {
					printTimelineEntry(entry, termWidth)
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

// printTimelineEntry renders a single entry right-aligned to the terminal width.
// Format: "      HH:MM:SS  <command><padding><duration>  <exit:N if non-zero>"
func printTimelineEntry(entry logfile.LogEntry, termWidth int) {
	const prefix = "      " // 6-space indent
	tsStr := formatTimeOnly(entry.Ts)
	cmdStr := strings.ReplaceAll(entry.Cmd, "\n", " ")

	// Format optional exit prefix and duration
	var exitPrefix string
	exitPrefixWidth := 0
	if entry.Exit != 0 {
		exitPrefix = display.Colorize(fmt.Sprintf("exit:%d", entry.Exit), display.Red) + "  "
		exitPrefixWidth = len(fmt.Sprintf("exit:%d", entry.Exit)) + 2 // "exit:N  "
	}

	durRaw := fmt.Sprintf("%gs", entry.Dur)
	durSlotWidth := 7
	if len(durRaw) > durSlotWidth {
		durSlotWidth = len(durRaw)
	}
	durPadded := fmt.Sprintf("%*s", durSlotWidth, durRaw)
	durStr := display.Colorize(durPadded, display.Dim)

	// Available width for command: total - prefix - "HH:MM:SS" - "  " - gutter(2) - exitPrefix - durSlot
	leftWidth := len(prefix) + 8 + 2 // "      " + "HH:MM:SS" + "  "
	rightWidth := exitPrefixWidth + durSlotWidth
	cmdBudget := termWidth - leftWidth - 2 - rightWidth // 2 = minimum gutter
	if cmdBudget < 10 {
		cmdBudget = 10
	}

	// Truncate command if needed
	cmdRunes := []rune(cmdStr)
	if len(cmdRunes) > cmdBudget {
		cmdStr = string(cmdRunes[:cmdBudget-1]) + "…"
	}

	// Calculate padding between command and right-side metadata
	usedWidth := leftWidth + len([]rune(cmdStr)) + rightWidth
	padding := termWidth - usedWidth
	if padding < 2 {
		padding = 2
	}

	fmt.Printf("%s%s  %s%s%s%s\n", prefix, tsStr, cmdStr, strings.Repeat(" ", padding), exitPrefix, durStr)
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
