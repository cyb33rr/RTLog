package cmd

import (
	"fmt"
	"os"

	"github.com/cyb33rr/rtlog/internal/export"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/spf13/cobra"
)

var exportOutput string

var exportCmd = &cobra.Command{
	Use:   "export <md|csv>",
	Short: "Export entries as Markdown or CSV",
	Long:  "Export log entries to a Markdown table or CSV file.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		format := args[0]
		if format != "md" && format != "csv" {
			fmt.Fprintf(os.Stderr, "Unknown export format: %s (use md or csv)\n", format)
			os.Exit(1)
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

		entries, err := d.LoadAll()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
			os.Exit(1)
		}

		if len(entries) == 0 {
			fmt.Fprintf(os.Stderr, "No entries in %s\n", logfile.EngagementName(path))
			return
		}

		var text string
		switch format {
		case "md":
			text = export.ExportMarkdown(entries)
		case "csv":
			text = export.ExportCSV(entries)
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
	rootCmd.AddCommand(exportCmd)
}
