package cmd

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/spf13/cobra"
)

type toolStat struct {
	count int
	ok    int
	fail  int
	dur   float64
}

type toolEntry struct {
	name string
	stat *toolStat
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Summary statistics for the engagement",
	Long:  "Show total commands, per-tool breakdown, success rate, and time span.",
	Run: func(cmd *cobra.Command, args []string) {
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

		total := len(entries)
		success := 0
		for _, e := range entries {
			if e.Exit == 0 {
				success++
			}
		}
		fail := total - success

		// Per-tool breakdown
		toolStats := map[string]*toolStat{}
		for _, entry := range entries {
			tool := entry.Tool
			if tool == "" {
				tool = "unknown"
			}
			ts, exists := toolStats[tool]
			if !exists {
				ts = &toolStat{}
				toolStats[tool] = ts
			}
			ts.count++
			if entry.Exit == 0 {
				ts.ok++
			} else {
				ts.fail++
			}
			ts.dur += entry.Dur
		}

		// Time span
		var epochs []int64
		for _, e := range entries {
			if e.Epoch > 0 {
				epochs = append(epochs, e.Epoch)
			}
		}
		spanStr := "N/A"
		if len(epochs) > 0 {
			minEpoch, maxEpoch := epochs[0], epochs[0]
			for _, ep := range epochs[1:] {
				if ep < minEpoch {
					minEpoch = ep
				}
				if ep > maxEpoch {
					maxEpoch = ep
				}
			}
			firstDt := time.Unix(minEpoch, 0).UTC()
			lastDt := time.Unix(maxEpoch, 0).UTC()
			span := lastDt.Sub(firstDt)
			spanStr = fmt.Sprintf("%s -> %s  (%s)",
				firstDt.Format("2006-01-02 15:04"),
				lastDt.Format("2006-01-02 15:04"),
				formatDuration(span),
			)
		}

		successRate := float64(0)
		if total > 0 {
			successRate = float64(success) / float64(total) * 100
		}

		engName := logfile.EngagementName(path)
		fmt.Println(display.Colorize(fmt.Sprintf("--- Stats for %s ---", engName), display.Bold))
		fmt.Println()
		fmt.Printf("  Total commands:  %d\n", total)
		fmt.Printf("  Success (exit 0): %s\n", display.Colorize(fmt.Sprintf("%d", success), display.Green))
		fmt.Printf("  Failed:           %s\n", display.Colorize(fmt.Sprintf("%d", fail), display.Red))
		fmt.Printf("  Success rate:     %.1f%%\n", successRate)
		fmt.Printf("  Time span:        %s\n", spanStr)
		fmt.Println()

		// Top 5 tools
		var toolList []toolEntry
		for name, ts := range toolStats {
			toolList = append(toolList, toolEntry{name, ts})
		}
		sort.Slice(toolList, func(i, j int) bool {
			return toolList[i].stat.count > toolList[j].stat.count
		})

		top5 := toolList
		if len(top5) > 5 {
			top5 = top5[:5]
		}
		fmt.Println(display.Colorize("  Top 5 tools:", display.Bold))
		for _, te := range top5 {
			printToolStat(te.name, te.stat)
		}

		// Full per-tool table sorted alphabetically
		fmt.Println()
		fmt.Println(display.Colorize("  All tools:", display.Bold))
		sort.Slice(toolList, func(i, j int) bool {
			return toolList[i].name < toolList[j].name
		})
		for _, te := range toolList {
			printToolStat(te.name, te.stat)
		}
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
}

func printToolStat(name string, ts *toolStat) {
	toolStr := display.Colorize(name, display.Cyan)
	padding := 30 - len(name)
	if padding < 1 {
		padding = 1
	}
	fmt.Printf("    %s%*s  count:%d  ok:%s  fail:%s  dur:%s\n",
		toolStr, padding, "",
		ts.count,
		display.Colorize(fmt.Sprintf("%d", ts.ok), display.Green),
		display.Colorize(fmt.Sprintf("%d", ts.fail), display.Red),
		display.Colorize(fmt.Sprintf("%.1fs", ts.dur), display.Dim),
	)
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
