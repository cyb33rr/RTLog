package export

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/cyb33rr/rtlog/internal/logfile"
)

var csvFields = []string{"ts", "epoch", "user", "host", "tty", "cwd", "tool", "cmd", "exit", "dur", "tag", "note", "out"}

// ExportCSV renders entries as CSV text.
func ExportCSV(entries []logfile.LogEntry) string {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	w.Write(csvFields)

	for _, e := range entries {
		row := []string{
			e.Ts,
			fmt.Sprintf("%d", e.Epoch),
			e.User,
			e.Host,
			e.TTY,
			e.Cwd,
			e.Tool,
			e.Cmd,
			fmt.Sprintf("%d", e.Exit),
			fmt.Sprintf("%g", e.Dur),
			e.Tag,
			e.Note,
			e.Out,
		}
		w.Write(row)
	}

	w.Flush()
	return strings.TrimRight(buf.String(), "\n")
}
