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

var rmYes bool

var rmCmd = &cobra.Command{
	Use:   "rm <engagement>",
	Short: "Delete an engagement and its database",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		if err := logfile.ValidateEngagementName(name); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid engagement name: %v\n", err)
			os.Exit(1)
		}

		dir := logfile.LogDir()
		dbPath := filepath.Join(dir, name+".db")

		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "error: engagement %q not found\n", name)
			os.Exit(1)
		}

		// Open DB to get count and force WAL checkpoint, then close
		d, err := db.Open(dir, name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer d.Close()

		count, err := d.Count()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		if !rmYes {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				fmt.Fprintln(os.Stderr, "Not a terminal. Use -y to confirm.")
				os.Exit(1)
			}
			fmt.Printf("Delete engagement %q (%d entries)? [y/N] ", name, count)
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(answer)) != "y" {
				fmt.Println("Aborted.")
				return
			}
		}

		// Remove DB and WAL sidecar files
		if err := os.Remove(dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")

		// Clear engagement from state if it was active
		st := state.ReadState()
		if st[state.KeyEngagement] == name {
			if _, err := state.UpdateState(map[string]string{state.KeyEngagement: ""}); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
				os.Exit(1)
			}
		}

		fmt.Printf("Deleted engagement %q (%d entries).\n", name, count)
	},
}

func init() {
	rmCmd.Flags().BoolVarP(&rmYes, "yes", "y", false, "skip confirmation prompt")
	rootCmd.AddCommand(rmCmd)
}
