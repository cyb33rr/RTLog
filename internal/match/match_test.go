package match

import (
	"os"
	"path/filepath"
	"testing"
)

func writeToolsConf(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "tools.conf")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadTools(t *testing.T) {
	p := writeToolsConf(t, "# comment\nnmap\ngobuster\nimpacket-*\n\n")
	m, err := LoadTools(p)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Match("nmap") {
		t.Error("expected nmap to match")
	}
	if !m.Match("gobuster") {
		t.Error("expected gobuster to match")
	}
	if !m.Match("impacket-smbexec") {
		t.Error("expected impacket-smbexec to match via glob")
	}
	if m.Match("vim") {
		t.Error("expected vim to NOT match")
	}
}

func TestLoadToolsEmpty(t *testing.T) {
	p := writeToolsConf(t, "# only comments\n\n")
	m, err := LoadTools(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Match("nmap") {
		t.Error("expected no matches with empty config")
	}
}

func TestLoadToolsWhitespace(t *testing.T) {
	p := writeToolsConf(t, "  nmap  \n\tgobuster\t\n")
	m, err := LoadTools(p)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Match("nmap") {
		t.Error("expected nmap to match after whitespace trim")
	}
	if !m.Match("gobuster") {
		t.Error("expected gobuster to match after tab trim")
	}
}

func TestExtractTool(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{"nmap -sV 10.10.10.5", "nmap"},
		{"sudo nmap -sV 10.10.10.5", "nmap"},
		{"sudo -u root nmap -sV 10.10.10.5", "nmap"},
		{"env FOO=bar nmap -sV 10.10.10.5", "nmap"},
		{"FOO=bar nmap -sV 10.10.10.5", "nmap"},
		{"time nmap -sV 10.10.10.5", "nmap"},
		{"nohup nmap -sV 10.10.10.5", "nmap"},
		{"/usr/bin/nmap -sV 10.10.10.5", "nmap"},
		{"proxychains nmap -sV 10.10.10.5", "nmap"},
		{"sudo proxychains nmap -sV 10.10.10.5", "nmap"},
		{"", ""},
		{"sudo", ""},
	}
	for _, tt := range tests {
		got := ExtractTool(tt.cmd)
		if got != tt.want {
			t.Errorf("ExtractTool(%q) = %q, want %q", tt.cmd, got, tt.want)
		}
	}
}
