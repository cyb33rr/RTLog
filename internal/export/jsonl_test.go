package export

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cyb33rr/rtlog/internal/logfile"
)

func TestExportJSONL_SingleEntry(t *testing.T) {
	entries := []logfile.LogEntry{
		{
			ID: 1, Ts: "2026-03-17T14:30:00Z", Epoch: 1742222400,
			User: "cyb3r", Host: "kali", TTY: "pts/0", Cwd: "/tmp",
			Tool: "nmap", Cmd: "nmap -sV 10.0.0.1", Exit: 0, Dur: 5.2,
			Tag: "recon", Note: "test", Out: "",
		},
	}

	result := ExportJSONL(entries)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Verify id is present (LogEntry has json:"-" on ID, so export must handle this)
	if id, ok := parsed["id"]; !ok {
		t.Error("missing 'id' field in JSONL output")
	} else if id.(float64) != 1 {
		t.Errorf("id = %v, want 1", id)
	}

	if parsed["tool"] != "nmap" {
		t.Errorf("tool = %v, want nmap", parsed["tool"])
	}
	if parsed["cmd"] != "nmap -sV 10.0.0.1" {
		t.Errorf("cmd = %v, want 'nmap -sV 10.0.0.1'", parsed["cmd"])
	}
}

func TestExportJSONL_MultipleEntries(t *testing.T) {
	entries := []logfile.LogEntry{
		{ID: 1, Ts: "2026-03-17T14:30:00Z", Epoch: 1742222400, User: "a", Host: "h", Cwd: "/", Tool: "nmap", Cmd: "nmap 10.0.0.1"},
		{ID: 2, Ts: "2026-03-17T14:31:00Z", Epoch: 1742222460, User: "a", Host: "h", Cwd: "/", Tool: "curl", Cmd: "curl http://10.0.0.1"},
	}

	result := ExportJSONL(entries)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}

	for i, line := range lines {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i, err)
		}
	}
}

func TestExportJSONL_Empty(t *testing.T) {
	result := ExportJSONL([]logfile.LogEntry{})
	if result != "" {
		t.Errorf("expected empty string for empty entries, got %q", result)
	}
}

func TestExportJSONL_SpecialChars(t *testing.T) {
	entries := []logfile.LogEntry{
		{ID: 1, Ts: "2026-03-17T14:30:00Z", Epoch: 1742222400, User: "a", Host: "h", Cwd: "/", Tool: "curl",
			Cmd: `curl -H "Authorization: Bearer token" http://10.0.0.1`, Note: "line1\nline2"},
	}

	result := ExportJSONL(entries)
	lines := strings.Split(strings.TrimSpace(result), "\n")

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("invalid JSON with special chars: %v", err)
	}
	if parsed["note"] != "line1\nline2" {
		t.Errorf("note not preserved: %v", parsed["note"])
	}
}
