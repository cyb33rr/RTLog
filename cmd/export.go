package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/cyb33rr/rtlog/internal/export"
	"github.com/cyb33rr/rtlog/internal/filter"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/spf13/cobra"
)

var (
	exportOutput string
	exportTool   string
	exportTag    string
	exportDate   string
	exportFrom   string
	exportTo     string
	exportFilter string
	exportRegex  string
)

var exportCmd = &cobra.Command{
	Use:   "export <md|csv|jsonl>",
	Short: "Export entries as Markdown, CSV, or JSONL",
	Long:  "Export log entries to a Markdown table, CSV, or JSONL file.\nSupports filtering by tool, tag, date range, substring, and regex.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		format := args[0]
		if format != "md" && format != "csv" && format != "jsonl" {
			fmt.Fprintf(os.Stderr, "Unknown export format: %s (use md, csv, or jsonl)\n", format)
			os.Exit(1)
		}

		// Validate mutual exclusivity: --date vs --from/--to
		if exportDate != "" && (exportFrom != "" || exportTo != "") {
			fmt.Fprintf(os.Stderr, "Cannot use --date with --from or --to\n")
			os.Exit(1)
		}

		// Validate date formats
		for _, pair := range []struct{ flag, val string }{
			{"--date", exportDate}, {"--from", exportFrom}, {"--to", exportTo},
		} {
			if pair.val != "" {
				if _, err := time.Parse("2006-01-02", pair.val); err != nil {
					fmt.Fprintf(os.Stderr, "Invalid %s format: %s (expected YYYY-MM-DD)\n", pair.flag, pair.val)
					os.Exit(1)
				}
			}
		}

		// Validate regex
		if exportRegex != "" {
			if _, err := regexp.Compile(exportRegex); err != nil {
				fmt.Fprintf(os.Stderr, "Invalid regex: %v\n", err)
				os.Exit(1)
			}
		}

		// Parse comma-separated tools and tags
		var tools, tags []string
		if exportTool != "" {
			tools = strings.Split(exportTool, ",")
		}
		if exportTag != "" {
			tags = strings.Split(exportTag, ",")
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

		hasFilters := len(tools) > 0 || len(tags) > 0 || exportDate != "" || exportFrom != "" || exportTo != "" || exportFilter != "" || exportRegex != ""

		entries, err := d.LoadFiltered(tools, tags, exportDate, exportFrom, exportTo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
			os.Exit(1)
		}

		// Apply substring filter
		if exportFilter != "" {
			entries = filter.MatchSubstring(entries, exportFilter)
		}

		// Apply regex filter
		if exportRegex != "" {
			entries, err = filter.MatchRegex(entries, exportRegex)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error applying regex: %v\n", err)
				os.Exit(1)
			}
		}

		if len(entries) == 0 {
			if hasFilters {
				fmt.Fprintf(os.Stderr, "No entries match the given filters\n")
			} else {
				fmt.Fprintf(os.Stderr, "No entries in %s\n", logfile.EngagementName(path))
			}
			return
		}

		var text string
		switch format {
		case "md":
			text = export.ExportMarkdown(entries)
		case "csv":
			text = export.ExportCSV(entries)
		case "jsonl":
			text = export.ExportJSONL(entries)
		}

		outPath := exportOutput
		if outPath == "" {
			outPath = logfile.EngagementName(path) + "." + format
		}

		if err := os.WriteFile(outPath, []byte(text+"\n"), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Written to %s\n", outPath)
	},
}

func init() {
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "output file path (default: <engagement>.<format>)")
	exportCmd.Flags().StringVar(&exportTool, "tool", "", "filter by tool (comma-separated, e.g. nmap,nxc)")
	exportCmd.Flags().StringVar(&exportTag, "tag", "", "filter by tag (comma-separated, e.g. recon,privesc)")
	exportCmd.Flags().StringVar(&exportDate, "date", "", "filter by date (YYYY-MM-DD)")
	exportCmd.Flags().StringVar(&exportFrom, "from", "", "filter from date inclusive (YYYY-MM-DD)")
	exportCmd.Flags().StringVar(&exportTo, "to", "", "filter to date inclusive (YYYY-MM-DD)")
	exportCmd.Flags().StringVar(&exportFilter, "filter", "", "filter by substring match across cmd, tool, cwd, tag, note")
	exportCmd.Flags().StringVarP(&exportRegex, "regex", "r", "", "filter by regex match across cmd, tool, cwd, tag, note")
	rootCmd.AddCommand(exportCmd)
}
