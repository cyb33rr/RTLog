package logfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadEntries_ValidJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	content := `{"ts":"2024-01-15T10:30:00Z","epoch":1705312200,"user":"tester","host":"kali","tty":"pts/1","cwd":"/tmp","tool":"nmap","cmd":"nmap -sV 10.10.10.5","exit":0,"dur":12.5,"tag":"recon","note":"","out":""}
{"ts":"2024-01-15T10:31:00Z","epoch":1705312260,"user":"tester","host":"kali","tty":"pts/1","cwd":"/tmp","tool":"curl","cmd":"curl http://10.10.10.5","exit":0,"dur":0.3,"tag":"recon","note":"test note","out":"response data"}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := LoadEntries(path, nil)
	if err != nil {
		t.Fatalf("LoadEntries failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].Tool != "nmap" {
		t.Errorf("expected tool 'nmap', got '%s'", entries[0].Tool)
	}
	if entries[0].Exit != 0 {
		t.Errorf("expected exit 0, got %d", entries[0].Exit)
	}
	if entries[0].Dur != 12.5 {
		t.Errorf("expected dur 12.5, got %f", entries[0].Dur)
	}
	if entries[1].Note != "test note" {
		t.Errorf("expected note 'test note', got '%s'", entries[1].Note)
	}
	if entries[1].Out != "response data" {
		t.Errorf("expected out 'response data', got '%s'", entries[1].Out)
	}
}

func TestLoadEntries_DateFilter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	content := `{"ts":"2024-01-15T10:30:00Z","epoch":1705312200,"user":"tester","host":"kali","tty":"pts/1","cwd":"/tmp","tool":"nmap","cmd":"nmap 10.10.10.5","exit":0,"dur":1,"tag":"","note":"","out":""}
{"ts":"2024-01-16T10:30:00Z","epoch":1705398600,"user":"tester","host":"kali","tty":"pts/1","cwd":"/tmp","tool":"curl","cmd":"curl http://10.10.10.5","exit":0,"dur":1,"tag":"","note":"","out":""}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	filterDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	entries, err := LoadEntries(path, &filterDate)
	if err != nil {
		t.Fatalf("LoadEntries failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after date filter, got %d", len(entries))
	}
	if entries[0].Tool != "nmap" {
		t.Errorf("expected tool 'nmap', got '%s'", entries[0].Tool)
	}
}

func TestLoadEntries_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")

	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := LoadEntries(path, nil)
	if err != nil {
		t.Fatalf("LoadEntries failed: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestLoadEntries_MalformedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "malformed.jsonl")

	content := `{"ts":"2024-01-15T10:30:00Z","tool":"nmap","cmd":"nmap 10.10.10.5","exit":0,"dur":1}
this is not json
{"ts":"2024-01-15T10:31:00Z","tool":"curl","cmd":"curl http://10.10.10.5","exit":0,"dur":1}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := LoadEntries(path, nil)
	if err != nil {
		t.Fatalf("LoadEntries failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (skipping malformed), got %d", len(entries))
	}
}

func TestLoadEntries_ControlCharRetry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ctrl.jsonl")

	// JSON with a control char that should be sanitized on retry
	content := "{\"ts\":\"2024-01-15T10:30:00Z\",\"tool\":\"nmap\",\"cmd\":\"nmap\x0110.10.10.5\",\"exit\":0,\"dur\":1}\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := LoadEntries(path, nil)
	if err != nil {
		t.Fatalf("LoadEntries failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after control char sanitization, got %d", len(entries))
	}
}

func TestEngagementName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/home/user/.rt/logs/test-engagement.jsonl", "test-engagement"},
		{"/home/user/.rt/logs/simple.jsonl", "simple"},
		{"foo.jsonl", "foo"},
	}

	for _, tc := range tests {
		got := EngagementName(tc.input)
		if got != tc.expected {
			t.Errorf("EngagementName(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestValidateEngagementName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"my-engagement", false},
		{"test_123", false},
		{"Project.2024", false},
		{"a", false},
		{"", true},
		{".", true},
		{"..", true},
		{"../../../etc/cron.d/evil", true},
		{"/etc/passwd", true},
		{".hidden", true},
		{"_leading-underscore", true},
		{"-leading-dash", true},
		{"has spaces", true},
		{"has\ttab", true},
	}

	for _, tc := range tests {
		err := ValidateEngagementName(tc.name)
		if (err != nil) != tc.wantErr {
			t.Errorf("ValidateEngagementName(%q) error = %v, wantErr %v", tc.name, err, tc.wantErr)
		}
	}
}

func TestCountEntries_LargeLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.jsonl")

	// Create a line exceeding the default 64KB scanner buffer
	bigOut := strings.Repeat("A", 100*1024)
	line := `{"ts":"2024-01-15T10:30:00Z","tool":"nmap","cmd":"nmap","exit":0,"dur":1,"out":"` + bigOut + `"}` + "\n"
	line += `{"ts":"2024-01-15T10:31:00Z","tool":"curl","cmd":"curl","exit":0,"dur":1}` + "\n"

	if err := os.WriteFile(path, []byte(line), 0644); err != nil {
		t.Fatal(err)
	}

	count := CountEntries(path)
	if count != 2 {
		t.Errorf("CountEntries with 100KB line: got %d, want 2", count)
	}
}

func TestToMap(t *testing.T) {
	entry := LogEntry{
		Ts:   "2024-01-15T10:30:00Z",
		Tool: "nmap",
		Cmd:  "nmap -sV 10.10.10.5",
		Exit: 0,
		Dur:  12.5,
		Tag:  "recon",
		Note: "test",
		Out:  "output",
	}

	m := ToMap(entry)
	if m["tool"] != "nmap" {
		t.Errorf("expected tool 'nmap', got '%v'", m["tool"])
	}
	if m["exit"] != 0 {
		t.Errorf("expected exit 0, got '%v'", m["exit"])
	}
	if m["dur"] != 12.5 {
		t.Errorf("expected dur 12.5, got '%v'", m["dur"])
	}
}
