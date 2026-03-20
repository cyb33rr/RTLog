package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cyb33rr/rtlog/internal/db"
	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import <file.jsonl> [file2.jsonl ...]",
	Short: "Import JSONL log files into SQLite databases",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		for _, path := range args {
			importFile(path)
		}
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
}

func importFile(path string) {
	base := filepath.Base(path)
	eng := strings.TrimSuffix(base, ".jsonl")
	if eng == base {
		fmt.Fprintf(os.Stderr, "skip: %s is not a .jsonl file\n", path)
		return
	}

	if err := logfile.ValidateEngagementName(eng); err != nil {
		fmt.Fprintf(os.Stderr, "skip: %s: invalid engagement name: %v\n", path, err)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
		return
	}
	defer f.Close()

	dir := logfile.LogDir()
	d, err := db.Open(dir, eng)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", eng, err)
		return
	}
	defer d.Close()

	existing := make(map[string]bool)
	entries, _ := d.LoadAll()
	for _, e := range entries {
		existing[dedupKey(e)] = true
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	var imported, skipped, malformed int

	for scanner.Scan() {
		var e logfile.LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			malformed++
			continue
		}
		key := dedupKey(e)
		if existing[key] {
			skipped++
			continue
		}
		if err := d.Insert(e); err != nil {
			display.Warn("insert failed: %v", err)
			continue
		}
		existing[key] = true
		imported++
	}

	if err := scanner.Err(); err != nil {
		display.Warn("error reading %s: %v", path, err)
	}

	fmt.Printf("Imported %d entries into engagement %q", imported, eng)
	if skipped > 0 {
		fmt.Printf(" (%d duplicates skipped)", skipped)
	}
	if malformed > 0 {
		fmt.Printf(" (%d malformed lines skipped)", malformed)
	}
	fmt.Println()
}

func dedupKey(e logfile.LogEntry) string {
	return fmt.Sprintf("%d\x00%s\x00%s\x00%s", e.Epoch, e.Cmd, e.Tool, e.Cwd)
}
