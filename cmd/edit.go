package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/spf13/cobra"
)

var editNote string
var editTag string

var editCmd = &cobra.Command{
	Use:   "edit <id> [--note TEXT] [--tag TEXT]",
	Short: "Edit note or tag on a log entry",
	Long:  "Modify the note or tag on an existing log entry. Other fields are immutable.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid id %q\n", args[0])
			os.Exit(1)
		}

		noteChanged := cmd.Flags().Changed("note")
		tagChanged := cmd.Flags().Changed("tag")

		if !noteChanged && !tagChanged {
			fmt.Fprintln(os.Stderr, "error: at least one of --note or --tag must be provided")
			cmd.Usage()
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

		fields := map[string]string{}
		if noteChanged {
			fields["note"] = editNote
		}
		if tagChanged {
			fields["tag"] = editTag
		}

		if err := d.Update(id, fields); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		// Show updated entry
		updated, err := d.GetByID(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		m := logfile.ToMap(*updated)
		fmt.Println(display.FmtEntry(m, int(updated.ID), 1))
	},
}

func init() {
	editCmd.Flags().StringVar(&editNote, "note", "", "new note text (empty to clear)")
	editCmd.Flags().StringVar(&editTag, "tag", "", "new tag text (empty to clear)")
	rootCmd.AddCommand(editCmd)
}
