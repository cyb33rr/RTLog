package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/cyb33rr/rtlog/internal/db"
	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/spf13/cobra"
)

var tailN int

var tailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Live-follow the log file",
	Long:  "Continuously watch the log for new entries (like tail -f).",
	Run: func(cmd *cobra.Command, args []string) {
		logPath := logfile.GetLogPath(engagementFlag)
		dir := filepath.Dir(logPath)
		eng := logfile.EngagementName(logPath)

		d, err := db.Open(dir, eng)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
			os.Exit(1)
		}

		entries, err := d.Tail(tailN)
		if err != nil {
			d.Close()
			fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
			os.Exit(1)
		}

		for _, e := range entries {
			fmt.Println(display.FmtEntry(logfile.ToMap(e), 0, 0))
		}

		var lastID int64
		if len(entries) > 0 {
			lastID = entries[len(entries)-1].ID
		}
		d.Close()

		// Now follow
		fmt.Println(display.Colorize(fmt.Sprintf("\n-- tailing %s (Ctrl+C to stop) --\n", eng), display.Dim))

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

		for {
			select {
			case <-sigCh:
				fmt.Println(display.Colorize("\nStopped.", display.Dim))
				return
			default:
			}

			time.Sleep(500 * time.Millisecond)

			d, err = db.Open(dir, eng)
			if err != nil {
				continue
			}
			newEntries, err := d.TailAfter(lastID)
			d.Close()
			if err != nil {
				continue
			}
			for _, e := range newEntries {
				fmt.Println(display.FmtEntry(logfile.ToMap(e), 0, 0))
				lastID = e.ID
			}
		}
	},
}

func init() {
	tailCmd.Flags().IntVarP(&tailN, "lines", "n", 20, "number of initial lines to display")
	rootCmd.AddCommand(tailCmd)
}
