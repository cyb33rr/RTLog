package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/spf13/cobra"

	"golang.org/x/term"
)

var deleteYes bool

var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a single log entry by ID",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid id %q\n", args[0])
			os.Exit(1)
		}

		d, err := openEngagementDB()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer d.Close()

		entry, err := d.GetByID(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if entry == nil {
			fmt.Fprintf(os.Stderr, "error: entry %d not found\n", id)
			os.Exit(1)
		}

		// Show the entry
		m := logfile.ToMap(*entry)
		fmt.Println(display.FmtEntry(m, int(entry.ID), 1))

		if !deleteYes {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				fmt.Fprintln(os.Stderr, "Not a terminal. Use -y to confirm.")
				os.Exit(1)
			}
			fmt.Printf("Delete entry %d? [y/N] ", id)
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(answer)) != "y" {
				fmt.Println("Aborted.")
				return
			}
		}

		if err := d.Delete(id); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Deleted entry %d.\n", id)
	},
}

func init() {
	deleteCmd.Flags().BoolVarP(&deleteYes, "yes", "y", false, "skip confirmation prompt")
	rootCmd.AddCommand(deleteCmd)
}
