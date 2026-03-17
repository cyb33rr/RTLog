package export

import (
	"encoding/json"
	"strings"

	"github.com/cyb33rr/rtlog/internal/logfile"
)

// jsonlEntry is a local struct that includes the ID field for JSONL export.
// LogEntry.ID has `json:"-"` which would omit it from encoding/json output.
type jsonlEntry struct {
	ID    int64   `json:"id"`
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

func toJSONLEntry(e logfile.LogEntry) jsonlEntry {
	return jsonlEntry{
		ID: e.ID, Ts: e.Ts, Epoch: e.Epoch, User: e.User, Host: e.Host,
		TTY: e.TTY, Cwd: e.Cwd, Tool: e.Tool, Cmd: e.Cmd, Exit: e.Exit,
		Dur: e.Dur, Tag: e.Tag, Note: e.Note, Out: e.Out,
	}
}

// ExportJSONL renders entries as newline-delimited JSON (one object per line).
func ExportJSONL(entries []logfile.LogEntry) string {
	if len(entries) == 0 {
		return ""
	}

	var b strings.Builder
	for _, e := range entries {
		data, err := json.Marshal(toJSONLEntry(e))
		if err != nil {
			continue
		}
		b.Write(data)
		b.WriteByte('\n')
	}

	return strings.TrimRight(b.String(), "\n")
}
