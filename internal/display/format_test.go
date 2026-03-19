package display

import (
	"strings"
	"testing"
)

// stripAnsi removes ANSI escape codes for test assertions.
func stripAnsi(s string) string {
	return RE_ANSI.ReplaceAllString(s, "")
}

func TestFmtCompactBasic(t *testing.T) {
	entry := Entry{
		"ts":   "2025-01-15T14:22:01Z",
		"cmd":  "nmap -sV 10.0.0.1",
		"exit": 0,
		"dur":  8.1,
		"tag":  "recon",
		"note": "port scan",
		"out":  "some output",
		"tool": "nmap",
		"cwd":  "/tmp",
	}
	got := stripAnsi(FmtCompact(entry))

	if !strings.Contains(got, "14:22:01") {
		t.Errorf("missing timestamp in %q", got)
	}
	if !strings.Contains(got, "nmap -sV 10.0.0.1") {
		t.Errorf("missing command in %q", got)
	}
	if !strings.Contains(got, "exit:0") {
		t.Errorf("missing exit code in %q", got)
	}
	if !strings.Contains(got, "8.1s") {
		t.Errorf("missing duration in %q", got)
	}
	if !strings.Contains(got, "[recon]") {
		t.Errorf("missing tag in %q", got)
	}
	if !strings.Contains(got, "# port scan") {
		t.Errorf("missing note in %q", got)
	}
	if !strings.Contains(got, "[+out]") {
		t.Errorf("missing [+out] in %q", got)
	}
}

func TestFmtCompactEmptyOptionalFields(t *testing.T) {
	entry := Entry{
		"ts":   "2025-01-15T10:00:00Z",
		"cmd":  "gobuster dir -u http://target",
		"exit": 1,
		"dur":  3.0,
		"tag":  "",
		"note": "",
		"out":  "",
		"tool": "gobuster",
		"cwd":  "/tmp",
	}
	got := stripAnsi(FmtCompact(entry))

	if strings.Contains(got, "[") {
		t.Errorf("should not have any brackets when tag and out are empty: %q", got)
	}
	if strings.Contains(got, "#") {
		t.Errorf("should not have note marker when note is empty: %q", got)
	}
}

func TestFmtCompactNewlinesCollapsed(t *testing.T) {
	entry := Entry{
		"ts":   "2025-01-15T10:00:00Z",
		"cmd":  "echo hello\necho world",
		"exit": 0,
		"dur":  0.1,
		"tag":  "",
		"note": "",
		"out":  "",
		"tool": "echo",
		"cwd":  "/tmp",
	}
	got := stripAnsi(FmtCompact(entry))

	if strings.Contains(got, "\n") {
		t.Errorf("command newlines should be collapsed: %q", got)
	}
	if !strings.Contains(got, "echo hello echo world") {
		t.Errorf("collapsed command not found in %q", got)
	}
}

func TestFmtCompactMissingTimestamp(t *testing.T) {
	entry := Entry{
		"ts":   "",
		"cmd":  "nmap -sV 10.0.0.1",
		"exit": 0,
		"dur":  1.0,
		"tag":  "",
		"note": "",
		"out":  "",
		"tool": "nmap",
		"cwd":  "/tmp",
	}
	got := stripAnsi(FmtCompact(entry))
	if !strings.Contains(got, "nmap") {
		t.Errorf("command missing from output: %q", got)
	}
}
