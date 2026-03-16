package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cyb33rr/rtlog/internal/logfile"
)

func resetLogFlags() {
	logCmdCmd = ""
	logCmdExit = 0
	logCmdDur = 0
	logCmdOut = ""
	logCmdTool = ""
	logCmdCwd = ""
	logCmdTag = ""
	logCmdNote = ""
}

func TestLogWritesEntry(t *testing.T) {
	resetLogFlags()

	tmpDir := t.TempDir()
	rtDir := filepath.Join(tmpDir, ".rt")
	logDir := filepath.Join(rtDir, "logs")
	os.MkdirAll(logDir, 0755)

	os.WriteFile(filepath.Join(rtDir, "state"), []byte("engagement=test-eng\ntag=recon\nnote=\nenabled=1\ncapture=1\n"), 0644)
	os.WriteFile(filepath.Join(rtDir, "tools.conf"), []byte("nmap\ngobuster\n"), 0644)

	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	rootCmd.SetArgs([]string{"log", "--cmd", "nmap -sV 10.10.10.5", "--exit", "0", "--dur", "5.2"})
	rootCmd.Execute()

	logPath := filepath.Join(logDir, "test-eng.jsonl")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}

	var entry logfile.LogEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("invalid JSONL: %v", err)
	}

	if entry.Tool != "nmap" {
		t.Errorf("tool = %q, want %q", entry.Tool, "nmap")
	}
	if entry.Cmd != "nmap -sV 10.10.10.5" {
		t.Errorf("cmd = %q, want %q", entry.Cmd, "nmap -sV 10.10.10.5")
	}
	if entry.Exit != 0 {
		t.Errorf("exit = %d, want 0", entry.Exit)
	}
	if entry.Tag != "recon" {
		t.Errorf("tag = %q, want %q", entry.Tag, "recon")
	}
}

func TestLogSkipsUnmatchedTool(t *testing.T) {
	resetLogFlags()

	tmpDir := t.TempDir()
	rtDir := filepath.Join(tmpDir, ".rt")
	logDir := filepath.Join(rtDir, "logs")
	os.MkdirAll(logDir, 0755)

	os.WriteFile(filepath.Join(rtDir, "state"), []byte("engagement=test-eng\ntag=\nnote=\nenabled=1\ncapture=1\n"), 0644)
	os.WriteFile(filepath.Join(rtDir, "tools.conf"), []byte("nmap\n"), 0644)

	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	rootCmd.SetArgs([]string{"log", "--cmd", "vim foo.txt"})
	rootCmd.Execute()

	logPath := filepath.Join(logDir, "test-eng.jsonl")
	if _, err := os.Stat(logPath); err == nil {
		t.Error("log file should not exist for unmatched tool")
	}
}
