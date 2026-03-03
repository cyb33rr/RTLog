package export

import (
	"fmt"
	"strings"

	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/cyb33rr/rtlog/internal/timeutil"
)

// ExportMarkdown renders entries as a Markdown table.
func ExportMarkdown(entries []logfile.LogEntry) string {
	var b strings.Builder

	b.WriteString("| Time | Tool | Command | Exit | Duration | Tag | Note | Output |\n")
	b.WriteString("|------|------|---------|------|----------|-----|------|--------|\n")

	for _, e := range entries {
		ts := formatTS(e.Ts)
		tool := e.Tool
		cmd := escapePipe(e.Cmd)
		cmd = strings.ReplaceAll(cmd, "`", "\\`")
		cmd = strings.ReplaceAll(cmd, "\n", "<br>")
		exit := fmt.Sprintf("%d", e.Exit)
		dur := fmt.Sprintf("%g", e.Dur)
		tag := e.Tag
		note := escapePipe(e.Note)
		out := escapePipe(strings.ReplaceAll(strings.TrimSpace(e.Out), "\n", "<br>"))

		fmt.Fprintf(&b, "| %s | %s | `%s` | %s | %ss | %s | %s | %s |\n",
			ts, tool, cmd, exit, dur, tag, note, out)
	}

	return strings.TrimRight(b.String(), "\n")
}

// formatTS converts an ISO timestamp to "YYYY-MM-DD HH:MM:SS".
func formatTS(ts string) string {
	if t, err := timeutil.Parse(ts); err == nil {
		return t.Format("2006-01-02 15:04:05")
	}
	return ts
}

// escapePipe replaces | with \| for Markdown table safety.
func escapePipe(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
