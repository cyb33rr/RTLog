package export

import (
	"fmt"
	"html"
	"strings"

	"github.com/cyb33rr/rtlog/internal/display"
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
		tool := escapePipe(e.Tool)
		cmd := html.EscapeString(e.Cmd)
		cmd = escapePipe(cmd)
		cmd = strings.ReplaceAll(cmd, "\n", "<br>")
		cmd = strings.ReplaceAll(cmd, "`", "\\`")
		exit := fmt.Sprintf("%d", e.Exit)
		dur := fmt.Sprintf("%g", e.Dur)
		tag := escapePipe(html.EscapeString(e.Tag))
		note := html.EscapeString(e.Note)
		note = escapePipe(strings.ReplaceAll(note, "\n", "<br>"))
		out := display.RE_ANSI.ReplaceAllString(strings.TrimSpace(e.Out), "")
		out = html.EscapeString(out)
		out = escapePipe(strings.ReplaceAll(out, "\n", "<br>"))

		fmt.Fprintf(&b, "| %s | %s | %s | %s | %ss | %s | %s | %s |\n",
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
