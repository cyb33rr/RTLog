package logfile

import (
	"testing"
)

func TestEngagementName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/home/user/.rt/logs/test-engagement.db", "test-engagement"},
		{"/home/user/.rt/logs/simple.db", "simple"},
		{"foo.db", "foo"},
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
