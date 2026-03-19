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
func FmtEntry(entry Entry, index, idxWidth int, showOutIndicator ...bool) string {
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
	hideOut := len(showOutIndicator) > 0 && !showOutIndicator[0]
	if !hideOut {
		if out, _ := entry["out"].(string); out != "" {
			outIndicator = Colorize(" [+out]", Dim)
		}
	}

	// Index
	idxStr := Colorize(fmt.Sprintf("%*d", idxWidth, index), Dim)

	return fmt.Sprintf("%s  %s  %s  %s  %s  %s  %s%s%s", idxStr, tsStr, toolStr, cmd, exitStr, durStr, tagStr, noteStr, outIndicator)
}

// FmtEntryHighlight formats an entry then highlights pattern matches.
func FmtEntryHighlight(entry Entry, pattern *regexp.Regexp, index, idxWidth int, showOutIndicator ...bool) string {
	line := FmtEntry(entry, index, idxWidth, showOutIndicator...)
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
// Format: <timestamp zone>  <command><padding><exit:N  Ns  [tag]  # note  [+out]>
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
	// exit(8) + "  " + dur(6) = 16 fixed chars before tag/note
	outIndicator := "       " // 7 spaces: same width as "[+out] "
	if out, _ := entry["out"].(string); out != "" {
		outIndicator = Colorize("[+out]", Dim) + " "
	}

	note := getString(entry, "note", "")
	var meta string
	if note != "" {
		// Note replaces exit+dur+tag, left-aligned to exit column, [+out] stays
		noteText := "# " + truncateText(note, 15)
		meta = fmt.Sprintf("%-18s", noteText) + outIndicator
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

		tag := getString(entry, "tag", "")
		tagStr := ""
		if tag != "" {
			tagStr = "  " + Colorize(fmt.Sprintf("[%s]", tag), Yellow)
		}

		meta = exitStr + "  " + durStr + tagStr + "  " + outIndicator
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
