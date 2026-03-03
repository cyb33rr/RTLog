package display

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cyb33rr/rtlog/internal/timeutil"
)

// LogEntry mirrors the fields needed for formatting. We accept a map to avoid
// circular imports with the logfile package.
type Entry = map[string]interface{}

// FmtEntry formats a single log entry for display.
// Format: idx  HH:MM:SS  tool  cmd  exit:N  Ns  [tag]  # note  [+out]
func FmtEntry(entry Entry, index, idxWidth int) string {
	// Timestamp
	tsRaw, _ := entry["ts"].(string)
	tsStr := formatTimestamp(tsRaw)

	// Tool
	tool := getString(entry, "tool", "?")
	toolStr := Colorize(tool, Cyan)

	// Command - collapse newlines
	cmd := getString(entry, "cmd", "")
	cmd = strings.ReplaceAll(cmd, "\n", " ")

	// Exit code
	exitCode := getInt(entry, "exit", -1)
	var exitStr string
	if exitCode == 0 {
		exitStr = Colorize(fmt.Sprintf("exit:%d", exitCode), Green)
	} else {
		exitStr = Colorize(fmt.Sprintf("exit:%d", exitCode), Red)
	}

	// Duration
	dur := getFloat(entry, "dur", 0)
	durStr := Colorize(fmt.Sprintf("%gs", dur), Dim)

	// Tag
	tag := getString(entry, "tag", "")
	tagStr := ""
	if tag != "" {
		tagStr = Colorize(fmt.Sprintf("[%s]", tag), Yellow)
	}

	// Note
	note := getString(entry, "note", "")
	noteStr := ""
	if note != "" {
		noteStr = fmt.Sprintf("  # %s", note)
	}

	// Output indicator
	outIndicator := ""
	if out, _ := entry["out"].(string); out != "" {
		outIndicator = Colorize(" [+out]", Dim)
	}

	// Index
	idxStr := Colorize(fmt.Sprintf("%*d", idxWidth, index), Dim)

	return fmt.Sprintf("%s  %s  %s  %s  %s  %s  %s%s%s", idxStr, tsStr, toolStr, cmd, exitStr, durStr, tagStr, noteStr, outIndicator)
}

// FmtEntryHighlight formats an entry then highlights pattern matches.
func FmtEntryHighlight(entry Entry, pattern *regexp.Regexp, index, idxWidth int) string {
	line := FmtEntry(entry, index, idxWidth)
	if pattern == nil {
		return line
	}
	if IsTTY {
		return pattern.ReplaceAllStringFunc(line, func(match string) string {
			return Magenta + Bold + match + Reset
		})
	}
	return line
}

// PrintOutputBlock prints captured output with separators.
func PrintOutputBlock(entry Entry, stripAnsi bool) {
	text, _ := entry["out"].(string)
	if text == "" || strings.TrimSpace(text) == "" {
		return
	}
	if stripAnsi {
		text = RE_ANSI.ReplaceAllString(text, "")
	}
	bar := Colorize("    --- output ---", Dim)
	end := Colorize("    --- end ---", Dim)
	fmt.Println(bar)
	for _, line := range strings.Split(text, "\n") {
		fmt.Printf("    %s\n", line)
	}
	fmt.Println(end)
}

// formatTimestamp extracts HH:MM:SS from an ISO timestamp.
func formatTimestamp(tsRaw string) string {
	if tsRaw == "" {
		return ""
	}
	if t, err := timeutil.Parse(tsRaw); err == nil {
		return t.Format("15:04:05")
	}
	// Fallback: first 8 chars
	if len(tsRaw) >= 8 {
		return tsRaw[:8]
	}
	return tsRaw
}

func getString(entry Entry, key, def string) string {
	if v, ok := entry[key].(string); ok {
		return v
	}
	return def
}

func getInt(entry Entry, key string, def int) int {
	switch v := entry[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return def
}

func getFloat(entry Entry, key string, def float64) float64 {
	switch v := entry[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return def
}
