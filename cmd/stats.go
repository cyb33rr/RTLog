package cmd

import (
	"fmt"
	"time"

	"github.com/cyb33rr/rtlog/internal/display"
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

// statsCmd is a hidden alias for statusCmd.
var statsCmd = &cobra.Command{
	Use:    "stats",
	Hidden: true,
	Short:  "Alias for status",
	Args:   cobra.NoArgs,
	Run:    runStatus,
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
