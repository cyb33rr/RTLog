package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/db"
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
			eng = st[state.KeyEngagement]
		}

		path := logfile.GetLogPath(eng)
		name := logfile.EngagementName(path)
		dir := filepath.Dir(path)

		d, err := db.Open(dir, name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
			os.Exit(1)
		}
		defer d.Close()

		count, err := d.Count()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error counting entries: %v\n", err)
			os.Exit(1)
		}

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

		if err := d.Clear(); err != nil {
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
