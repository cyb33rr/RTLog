package cmd

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/cyb33rr/rtlog/internal/state"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show state overview and engagement statistics",
	Long: `Show current rtlog state (engagement, tag, note, logging, capture)
and, if an engagement is active, summary statistics including command counts,
success rate, time span, and per-tool breakdown.`,
	Args: cobra.NoArgs,
	Run:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) {
	st := state.ReadState()

	eng := st[state.KeyEngagement]
	engDisplay := eng
	if engDisplay == "" {
		engDisplay = "(none)"
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
	fmt.Printf("  Engagement:  %s\n", display.Colorize(engDisplay, display.Cyan))
	fmt.Printf("  Tag:         %s\n", display.Colorize(tag, display.Yellow))
	fmt.Printf("  Note:        %s\n", note)
	fmt.Printf("  Logging:     %s\n", display.Colorize(enabled, enabledColor))
	fmt.Printf("  Capture:     %s\n", display.Colorize(capture, captureColor))

	// If no engagement is set, skip stats section
	if eng == "" {
		return
	}

	path, err := logfile.GetLogPath(engagementFlag)
	if err != nil {
		return
	}

	d, err := openEngagementDB()
	if err != nil {
		return
	}
	defer d.Close()

	entries, err := d.LoadAll()
	if err != nil || len(entries) == 0 {
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
	fmt.Println()
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
}
