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
// Format: idx  HH:MM:SS  tool  cmd  exit:N  Ns  [tag]  # note
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

	// Index
	idxStr := Colorize(fmt.Sprintf("%*d", idxWidth, index), Dim)

	return fmt.Sprintf("%s  %s  %s  %s  %s  %s  %s%s", idxStr, tsStr, toolStr, cmd, exitStr, durStr, tagStr, noteStr)
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

// PrintOutputBlock prints captured output indented under the entry.
func PrintOutputBlock(entry Entry, stripAnsi bool) {
	text, _ := entry["out"].(string)
	if text == "" || strings.TrimSpace(text) == "" {
		return
	}
	if stripAnsi {
		text = RE_ANSI.ReplaceAllString(text, "")
	}
	text = strings.TrimRight(text, "\n")
	fmt.Println()
	for _, line := range strings.Split(text, "\n") {
		fmt.Printf("    %s\n", line)
	}
	fmt.Println()
}

// FmtCompact formats an entry as a compact single line for the Atuin-style TUI.
// Width is the terminal width used to right-align metadata and truncate the command.
// When metadata alone exceeds the available width, the minimum 10-char command budget
// and 2-char gutter are enforced; the caller's render loop is expected to clip the result.
// Format: <timestamp zone>  <command><padding><exit:N  Ns  [tag(10)]  |  # note  [tag(10)]>
func FmtCompact(entry Entry, width int) string {
	// Timestamp zone: always 10 visible chars (8-char time + 2-space gap, or 10 spaces)
	tsRaw, _ := entry["ts"].(string)
	ts := formatTimestamp(tsRaw)
	var tsZone string
	if ts == "" {
		tsZone = "          " // 10 spaces
	} else {
		tsZone = ts + "  "
	}

	// Command — collapse newlines (plain text, no ANSI)
	cmd := getString(entry, "cmd", "")
	cmd = strings.ReplaceAll(cmd, "\n", " ")

	// Build metadata suffix
	// exit(8) + "  " + dur(6) = 16 fixed chars before tag slot
	tag := getString(entry, "tag", "")
	var tagSlot string
	if tag != "" {
		// Truncate tag to fit in 8 chars (10 - 2 for brackets), pad to 10 visible
		tagText := truncateText(tag, 8)
		raw := fmt.Sprintf("[%s]", tagText)
		tagSlot = Colorize(raw, Yellow) + strings.Repeat(" ", 10-len([]rune(raw)))
	} else {
		tagSlot = "          " // 10 spaces
	}

	note := getString(entry, "note", "")
	var meta string
	if note != "" {
		// Note replaces exit+dur, left-aligned to exit column; tag slot stays
		noteText := "# " + truncateText(note, 15)
		meta = fmt.Sprintf("%-18s", noteText) + tagSlot
	} else {
		// No note: show full metadata
		exitCode := getInt(entry, "exit", -1)
		exitRaw := fmt.Sprintf("%-8s", fmt.Sprintf("exit:%d", exitCode))
		var exitStr string
		if exitCode == 0 {
			exitStr = Colorize(exitRaw, Green)
		} else {
			exitStr = Colorize(exitRaw, Red)
		}

		dur := getFloat(entry, "dur", 0)
		durStr := Colorize(fmt.Sprintf("%-6s", fmt.Sprintf("%gs", dur)), Dim)

		meta = exitStr + "  " + durStr + "  " + tagSlot
	}
	metaWidth := visibleLen(meta)

	// Command width budget: total - timestamp(10) - gutter(2) - metadata
	cmdWidth := width - 10 - 2 - metaWidth
	if cmdWidth < 10 {
		cmdWidth = 10
	}

	cmd = truncateText(cmd, cmdWidth)

	// Pad between command and metadata to right-align
	usedWidth := 10 + visibleLen(cmd) + metaWidth
	padding := width - usedWidth
	if padding < 2 {
		padding = 2 // minimum gutter
	}

	return tsZone + cmd + strings.Repeat(" ", padding) + meta
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

// visibleLen returns the number of visible runes in s, excluding ANSI escape sequences.
func visibleLen(s string) int {
	return len([]rune(RE_ANSI.ReplaceAllString(s, "")))
}

// truncateText truncates plain text (no ANSI) to max visible runes, appending "…" if truncated.
func truncateText(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 0 {
		return "…"
	}
	return string(runes[:max-1]) + "…"
}
