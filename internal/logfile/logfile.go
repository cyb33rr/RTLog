package logfile

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cyb33rr/rtlog/internal/timeutil"
)

// LogEntry represents a single logged command.
type LogEntry struct {
	Ts    string  `json:"ts"`
	Epoch int64   `json:"epoch"`
	User  string  `json:"user"`
	Host  string  `json:"host"`
	TTY   string  `json:"tty"`
	Cwd   string  `json:"cwd"`
	Tool  string  `json:"tool"`
	Cmd   string  `json:"cmd"`
	Exit  int     `json:"exit"`
	Dur   float64 `json:"dur"`
	Tag   string  `json:"tag"`
	Note  string  `json:"note"`
	Out   string  `json:"out"`
}

// RE_JSON_CTRL matches JSON-illegal control characters (except \t, \n, \r).
var RE_JSON_CTRL = regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f]`)

// LogDir returns the log directory path (~/.rt/logs/).
func LogDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".rt", "logs")
	}
	return filepath.Join(home, ".rt", "logs")
}

// LoadEntries reads JSONL entries from path, optionally filtering by date.
// Malformed lines are skipped with a warning on stderr.
func LoadEntries(path string, dateFilter *time.Time) ([]LogEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(f)
	// Allow up to 10MB lines for entries with large captured output.
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	lineno := 0

	for scanner.Scan() {
		lineno++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Retry after stripping stray control chars
			sanitized := RE_JSON_CTRL.ReplaceAllString(line, "")
			if err2 := json.Unmarshal([]byte(sanitized), &entry); err2 != nil {
				fmt.Fprintf(os.Stderr, "Warning: skipping malformed line %d in %s: %v\n", lineno, path, err2)
				continue
			}
		}

		if dateFilter != nil {
			entryDate, err := parseDate(entry.Ts)
			if err != nil {
				continue
			}
			filterDate := dateFilter.Format("2006-01-02")
			if entryDate != filterDate {
				continue
			}
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return entries, err
	}

	return entries, nil
}

// parseDate extracts the YYYY-MM-DD portion from an ISO timestamp.
func parseDate(ts string) (string, error) {
	if ts == "" {
		return "", fmt.Errorf("empty timestamp")
	}
	if t, err := timeutil.Parse(ts); err == nil {
		return t.Format("2006-01-02"), nil
	}
	// Try extracting date prefix directly
	if len(ts) >= 10 {
		return ts[:10], nil
	}
	return "", fmt.Errorf("cannot parse timestamp: %s", ts)
}

// engagementInfo holds path and mtime for sorting.
type engagementInfo struct {
	Path  string
	Mtime time.Time
}

// AvailableEngagements returns .jsonl file paths sorted by mtime (newest first).
func AvailableEngagements() []string {
	dir := LogDir()
	matches, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil || len(matches) == 0 {
		return nil
	}

	infos := make([]engagementInfo, 0, len(matches))
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		infos = append(infos, engagementInfo{Path: m, Mtime: info.ModTime()})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Mtime.After(infos[j].Mtime)
	})

	result := make([]string, len(infos))
	for i, info := range infos {
		result[i] = info.Path
	}
	return result
}

// GetLogPath resolves an engagement name to its .jsonl file path.
// If engagement is empty, returns the most recently modified file.
// Prints diagnostics on failure and calls os.Exit(1).
func GetLogPath(engagement string) string {
	dir := LogDir()

	if engagement != "" {
		// Try direct path first (O(1) instead of listing all files)
		candidate := filepath.Join(dir, engagement+".jsonl")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}

		// Not found - list available for error message
		files := AvailableEngagements()
		fmt.Fprintf(os.Stderr, "Engagement '%s' not found.\n", engagement)
		if len(files) > 0 {
			fmt.Fprintln(os.Stderr, "Available engagements:")
			for _, f := range files {
				fmt.Fprintf(os.Stderr, "  %s\n", EngagementName(f))
			}
		}
		os.Exit(1)
	}

	files := AvailableEngagements()
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "No .jsonl log files found in %s\n", dir)
		fmt.Fprintln(os.Stderr, "Create one with: rtlog new <name>")
		os.Exit(1)
	}

	return files[0]
}

// CountEntries counts non-empty lines in a JSONL file without deserializing.
func CountEntries(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			count++
		}
	}
	return count
}

// EngagementName extracts the stem name from a .jsonl path.
func EngagementName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".jsonl")
}

// ToMap converts a LogEntry to a map for use with display functions.
func ToMap(e LogEntry) map[string]interface{} {
	return map[string]interface{}{
		"ts":    e.Ts,
		"epoch": e.Epoch,
		"user":  e.User,
		"host":  e.Host,
		"tty":   e.TTY,
		"cwd":   e.Cwd,
		"tool":  e.Tool,
		"cmd":   e.Cmd,
		"exit":  e.Exit,
		"dur":   e.Dur,
		"tag":   e.Tag,
		"note":  e.Note,
		"out":   e.Out,
	}
}
