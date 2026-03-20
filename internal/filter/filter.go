package filter

import (
	"regexp"
	"strings"

	"github.com/cyb33rr/rtlog/internal/logfile"
)

// fields returns the 5 searchable field values from a LogEntry.
func fields(e logfile.LogEntry) [5]string {
	return [5]string{e.Cmd, e.Tool, e.Cwd, e.Tag, e.Note}
}

// MatchSubstring returns entries where any of the 5 searchable fields
// contains substr (case-insensitive).
func MatchSubstring(entries []logfile.LogEntry, substr string) []logfile.LogEntry {
	lower := strings.ToLower(substr)
	var result []logfile.LogEntry
	for _, e := range entries {
		for _, f := range fields(e) {
			if strings.Contains(strings.ToLower(f), lower) {
				result = append(result, e)
				break
			}
		}
	}
	if result == nil {
		result = []logfile.LogEntry{}
	}
	return result
}

// MatchRegex returns entries where any of the 5 searchable fields
// matches the given regex pattern. Returns an error if the pattern is invalid.
func MatchRegex(entries []logfile.LogEntry, pattern string) ([]logfile.LogEntry, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	var result []logfile.LogEntry
	for _, e := range entries {
		for _, f := range fields(e) {
			if re.MatchString(f) {
				result = append(result, e)
				break
			}
		}
	}
	if result == nil {
		result = []logfile.LogEntry{}
	}
	return result, nil
}
