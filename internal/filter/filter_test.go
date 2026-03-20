package filter

import (
	"testing"

	"github.com/cyb33rr/rtlog/internal/logfile"
)

func sampleEntries() []logfile.LogEntry {
	return []logfile.LogEntry{
		{Cmd: "nmap -sV 10.0.0.1", Tool: "nmap", Cwd: "/tmp", Tag: "recon", Note: "initial scan"},
		{Cmd: "gobuster dir -u http://target", Tool: "gobuster", Cwd: "/opt", Tag: "recon", Note: ""},
		{Cmd: "evil-winrm -i 10.0.0.1", Tool: "evil-winrm", Cwd: "/tmp", Tag: "exploitation", Note: "got shell"},
	}
}

func TestMatchSubstringByCmd(t *testing.T) {
	result := MatchSubstring(sampleEntries(), "10.0.0.1")
	if len(result) != 2 {
		t.Errorf("got %d, want 2", len(result))
	}
}

func TestMatchSubstringByTool(t *testing.T) {
	result := MatchSubstring(sampleEntries(), "gobuster")
	if len(result) != 1 {
		t.Errorf("got %d, want 1", len(result))
	}
}

func TestMatchSubstringByCwd(t *testing.T) {
	result := MatchSubstring(sampleEntries(), "/opt")
	if len(result) != 1 {
		t.Errorf("got %d, want 1", len(result))
	}
}

func TestMatchSubstringByTag(t *testing.T) {
	result := MatchSubstring(sampleEntries(), "exploitation")
	if len(result) != 1 {
		t.Errorf("got %d, want 1", len(result))
	}
}

func TestMatchSubstringByNote(t *testing.T) {
	result := MatchSubstring(sampleEntries(), "shell")
	if len(result) != 1 {
		t.Errorf("got %d, want 1", len(result))
	}
}

func TestMatchSubstringCaseInsensitive(t *testing.T) {
	result := MatchSubstring(sampleEntries(), "NMAP")
	if len(result) != 1 {
		t.Errorf("got %d, want 1 (case-insensitive match on entry 0)", len(result))
	}
}

func TestMatchSubstringNoMatch(t *testing.T) {
	result := MatchSubstring(sampleEntries(), "zzzznotfound")
	if len(result) != 0 {
		t.Errorf("got %d, want 0", len(result))
	}
}

func TestMatchRegexByCmd(t *testing.T) {
	result, err := MatchRegex(sampleEntries(), `10\.0\.0\.\d+`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d, want 2", len(result))
	}
}

func TestMatchRegexByTool(t *testing.T) {
	result, err := MatchRegex(sampleEntries(), `^gobuster$`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d, want 1", len(result))
	}
}

func TestMatchRegexByCwd(t *testing.T) {
	result, err := MatchRegex(sampleEntries(), `/opt`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d, want 1", len(result))
	}
}

func TestMatchRegexByTag(t *testing.T) {
	result, err := MatchRegex(sampleEntries(), `exploit.*`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d, want 1", len(result))
	}
}

func TestMatchRegexByNote(t *testing.T) {
	result, err := MatchRegex(sampleEntries(), `got\s+shell`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d, want 1", len(result))
	}
}

func TestMatchRegexNoMatch(t *testing.T) {
	result, err := MatchRegex(sampleEntries(), `zzzznotfound`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("got %d, want 0", len(result))
	}
}

func TestMatchRegexInvalidPattern(t *testing.T) {
	_, err := MatchRegex(sampleEntries(), `[invalid`)
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestMatchRegexEmptyPattern(t *testing.T) {
	result, err := MatchRegex(sampleEntries(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("got %d, want 3 (empty pattern matches all)", len(result))
	}
}
