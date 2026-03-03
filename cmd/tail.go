package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/spf13/cobra"
)

var tailN int

var tailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Live-follow the log file",
	Long:  "Continuously watch the log file for new entries (like tail -f).",
	Run: func(cmd *cobra.Command, args []string) {
		path := logfile.GetLogPath(engagementFlag)

		// Read all lines to show the last N
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening log: %v\n", err)
			os.Exit(1)
		}

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
		var allLines []string
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				allLines = append(allLines, line)
			}
		}
		f.Close()

		// Show last N entries
		start := 0
		if len(allLines) > tailN {
			start = len(allLines) - tailN
		}
		tailLines := allLines[start:]
		for _, line := range tailLines {
			var entry logfile.LogEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}
			m := logfile.ToMap(entry)
			fmt.Println(display.FmtEntry(m, 0, 0))
		}

		// Now follow
		engName := logfile.EngagementName(path)
		fmt.Println(display.Colorize(fmt.Sprintf("\n-- tailing %s (Ctrl+C to stop) --\n", engName), display.Dim))

		// Handle SIGINT gracefully
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		// Reopen file and seek to end
		fTail, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening log for tailing: %v\n", err)
			os.Exit(1)
		}
		defer fTail.Close()

		// Seek to end and record position
		pos, _ := fTail.Seek(0, io.SeekEnd)

		tailScanner := bufio.NewScanner(fTail)
		tailScanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

		for {
			select {
			case <-sigCh:
				fmt.Println(display.Colorize("\nStopped.", display.Dim))
				return
			default:
				// Detect file truncation (e.g. after rtlog clear)
				if info, err := fTail.Stat(); err == nil && info.Size() < pos {
					fTail.Seek(0, io.SeekStart)
					pos = 0
					tailScanner = bufio.NewScanner(fTail)
					tailScanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
				}

				if tailScanner.Scan() {
					line := strings.TrimSpace(tailScanner.Text())
					// Track position after successful read
					pos += int64(len(tailScanner.Bytes())) + 1
					if line == "" {
						continue
					}
					var entry logfile.LogEntry
					if err := json.Unmarshal([]byte(line), &entry); err != nil {
						continue
					}
					m := logfile.ToMap(entry)
					fmt.Println(display.FmtEntry(m, 0, 0))
				} else {
					// No new data, sleep and retry
					time.Sleep(500 * time.Millisecond)
				}
			}
		}
	},
}

func init() {
	tailCmd.Flags().IntVarP(&tailN, "lines", "n", 20, "number of initial lines to display")
	rootCmd.AddCommand(tailCmd)
}
