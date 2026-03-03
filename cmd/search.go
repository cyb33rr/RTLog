package cmd

import (
	"fmt"
	"os"
	"regexp"

	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <keyword>",
	Short: "Search entries by keyword",
	Long:  "Case-insensitive search across cmd, tool, cwd, tag, note, user, and host fields.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		keyword := args[0]
		path := logfile.GetLogPath(engagementFlag)
		entries, err := logfile.LoadEntries(path, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
			os.Exit(1)
		}

		pattern := regexp.MustCompile("(?i)" + regexp.QuoteMeta(keyword))

		var matches []logfile.LogEntry
		for _, entry := range entries {
			for _, val := range []string{entry.Cmd, entry.Tool, entry.Cwd, entry.Tag, entry.Note, entry.User, entry.Host} {
				if val != "" && pattern.MatchString(val) {
					matches = append(matches, entry)
					break
				}
			}
		}

		engName := logfile.EngagementName(path)
		if len(matches) == 0 {
			fmt.Printf("No matches for '%s' in %s\n", keyword, engName)
			return
		}

		header := fmt.Sprintf("--- %d match(es) for '%s' in %s ---", len(matches), keyword, engName)
		fmt.Println(display.Colorize(header, display.Bold))
		fmt.Println()

		idxWidth := len(fmt.Sprintf("%d", len(matches)))
		for i, entry := range matches {
			m := logfile.ToMap(entry)
			fmt.Println(display.FmtEntryHighlight(m, pattern, i+1, idxWidth))
		}

		// Print match count summary
		fmt.Println()
		fmt.Printf("%s\n", display.Colorize(
			fmt.Sprintf("%d result(s)", len(matches)),
			display.Dim,
		))
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
