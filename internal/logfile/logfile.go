package logfile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// LogEntry represents a single logged command.
type LogEntry struct {
	ID    int64   `json:"-"`
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

// reEngagementName allows alphanumeric, dots, hyphens, underscores; must start with alnum.
var reEngagementName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// ValidateEngagementName checks that a name is safe for use as a filename.
// Rejects empty strings, ".", "..", path separators, and names that don't
// match the allowed pattern.
func ValidateEngagementName(name string) error {
	if name == "" {
		return fmt.Errorf("engagement name cannot be empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("engagement name cannot be '.' or '..'")
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("engagement name cannot contain path separators")
	}
	if !reEngagementName.MatchString(name) {
		return fmt.Errorf("engagement name must start with alphanumeric and contain only [a-zA-Z0-9._-]")
	}
	return nil
}

// LogDir returns the log directory path (~/.rt/logs/).
func LogDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot determine home directory: %v\n", err)
		return filepath.Join(".", ".rt", "logs")
	}
	return filepath.Join(home, ".rt", "logs")
}

// engagementInfo holds path and mtime for sorting.
type engagementInfo struct {
	Path  string
	Mtime time.Time
}

// AvailableEngagements returns .db file paths sorted by mtime (newest first).
func AvailableEngagements() []string {
	dir := LogDir()
	matches, err := filepath.Glob(filepath.Join(dir, "*.db"))
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

// GetLogPath resolves an engagement name to its .db file path.
// If engagement is empty, returns the most recently modified file.
// Returns an error with diagnostic information on failure.
func GetLogPath(engagement string) (string, error) {
	dir := LogDir()

	if engagement != "" {
		// Try direct path first (O(1) instead of listing all files)
		candidate := filepath.Join(dir, engagement+".db")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		// Not found - list available for error message
		files := AvailableEngagements()
		msg := fmt.Sprintf("engagement '%s' not found", engagement)
		if len(files) > 0 {
			names := make([]string, len(files))
			for i, f := range files {
				names[i] = EngagementName(f)
			}
			msg += fmt.Sprintf("; available engagements: %s", strings.Join(names, ", "))
		}
		return "", fmt.Errorf("%s", msg)
	}

	files := AvailableEngagements()
	if len(files) == 0 {
		return "", fmt.Errorf("no log databases found in %s\nCreate one with: rtlog new <name>", dir)
	}

	return files[0], nil
}

// EngagementName extracts the stem name from a .db path.
func EngagementName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".db")
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
