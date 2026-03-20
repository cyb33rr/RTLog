package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/db"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/cyb33rr/rtlog/internal/state"
)

var renameCmd = &cobra.Command{
	Use:   "rename <old> <new>",
	Short: "Rename an engagement",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		oldName := args[0]
		newName := args[1]

		if err := logfile.ValidateEngagementName(oldName); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid engagement name %q: %v\n", oldName, err)
			os.Exit(1)
		}
		if err := logfile.ValidateEngagementName(newName); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid engagement name %q: %v\n", newName, err)
			os.Exit(1)
		}

		dir := logfile.LogDir()
		oldPath := filepath.Join(dir, oldName+".db")
		newPath := filepath.Join(dir, newName+".db")

		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "error: engagement %q not found\n", oldName)
			os.Exit(1)
		}

		if _, err := os.Stat(newPath); err == nil {
			fmt.Fprintf(os.Stderr, "error: engagement %q already exists\n", newName)
			os.Exit(1)
		}

		// Open and close DB to force WAL checkpoint
		d, err := db.Open(dir, oldName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer d.Close()

		// Rename main DB file
		if err := os.Rename(oldPath, newPath); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		// Rename WAL sidecars if they still exist
		os.Rename(oldPath+"-wal", newPath+"-wal")
		os.Rename(oldPath+"-shm", newPath+"-shm")

		// Update state if active engagement was renamed
		st := state.ReadState()
		if st[state.KeyEngagement] == oldName {
			if _, err := state.UpdateState(map[string]string{state.KeyEngagement: newName}); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
				os.Exit(1)
			}
		}

		fmt.Printf("[rtlog] Renamed: %s -> %s\n", oldName, newName)
	},
}

func init() {
	rootCmd.AddCommand(renameCmd)
}
