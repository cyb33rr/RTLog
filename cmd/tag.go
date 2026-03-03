package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/cyb33rr/rtlog/internal/state"
)

var tagClear bool
var tagList bool

var tagCmd = &cobra.Command{
	Use:   "tag [name]",
	Short: "Show, set, or clear the current tag",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if tagClear {
			if _, err := state.UpdateState(map[string]string{state.KeyTag: ""}); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("[rtlog] Tag cleared")
			return
		}

		if tagList {
			listTags()
			return
		}

		if len(args) > 0 {
			if _, err := state.UpdateState(map[string]string{state.KeyTag: args[0]}); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("[rtlog] Tag set: %s\n", args[0])
			return
		}

		// No args: show current tag
		st := state.ReadState()
		tag := st[state.KeyTag]
		if tag == "" {
			tag = "(none)"
		}
		fmt.Printf("[rtlog] Tag: %s\n", tag)
	},
}

func listTags() {
	logPath := logfile.GetLogPath(engagementFlag)

	entries, err := logfile.LoadEntries(logPath, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
		os.Exit(1)
	}

	counts := make(map[string]int)
	for _, e := range entries {
		tag := e.Tag
		if tag == "" {
			tag = "untagged"
		}
		counts[tag]++
	}

	// Sort by count descending, then name ascending
	type tagCount struct {
		Tag   string
		Count int
	}
	sorted := make([]tagCount, 0, len(counts))
	for tag, count := range counts {
		sorted = append(sorted, tagCount{tag, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Count != sorted[j].Count {
			return sorted[i].Count > sorted[j].Count
		}
		return sorted[i].Tag < sorted[j].Tag
	})

	fmt.Println(display.Colorize(fmt.Sprintf("--- Tags in %s ---", logfile.EngagementName(logPath)), display.Bold))
	for _, tc := range sorted {
		fmt.Printf("  %8s  %s\n", display.Colorize(fmt.Sprintf("%d", tc.Count), display.Cyan), tc.Tag)
	}
}

func init() {
	tagCmd.Flags().BoolVar(&tagClear, "clear", false, "clear the current tag")
	tagCmd.Flags().BoolVar(&tagList, "list", false, "list all tags with counts")
	rootCmd.AddCommand(tagCmd)
}
