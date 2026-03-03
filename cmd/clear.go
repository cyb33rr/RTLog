package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/cyb33rr/rtlog/internal/state"

	"golang.org/x/term"
)

var clearYes bool

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all entries from the current engagement log",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// Resolve engagement
		eng := engagementFlag
		if eng == "" {
			st := state.ReadState()
			eng = st["engagement"]
		}

		path := logfile.GetLogPath(eng)
		entries, err := logfile.LoadEntries(path, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
			os.Exit(1)
		}

		count := len(entries)
		name := logfile.EngagementName(path)

		if count == 0 {
			fmt.Printf("Log %s is already empty.\n", name)
			return
		}

		if !clearYes {
			// If not a TTY, require -y
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				fmt.Fprintln(os.Stderr, "Not a terminal. Use -y to confirm.")
				os.Exit(1)
			}
			fmt.Printf("Clear %d entries from %s? [y/N] ", count, name)
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(answer)) != "y" {
				fmt.Println("Aborted.")
				return
			}
		}

		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error clearing log: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Cleared %d entries from %s.\n", count, name)
	},
}

func init() {
	clearCmd.Flags().BoolVarP(&clearYes, "yes", "y", false, "skip confirmation prompt")
	rootCmd.AddCommand(clearCmd)
}
