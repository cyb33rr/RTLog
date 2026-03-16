package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cyb33rr/rtlog/internal/db"
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
	logCmdOutFile = ""
	logCmdTTY = ""
}

func TestLogWritesEntry(t *testing.T) {
	resetLogFlags()

	tmpDir := t.TempDir()
	rtDir := filepath.Join(tmpDir, ".rt")
	logDir := filepath.Join(rtDir, "logs")
	os.MkdirAll(logDir, 0755)

	os.WriteFile(filepath.Join(rtDir, "state"), []byte("engagement=test-eng\ntag=recon\nnote=\nenabled=1\ncapture=1\n"), 0644)
	os.WriteFile(filepath.Join(rtDir, "tools.conf"), []byte("nmap\ngobuster\n"), 0644)

	t.Setenv("HOME", tmpDir)

	rootCmd.SetArgs([]string{"log", "--cmd", "nmap -sV 10.10.10.5", "--exit", "0", "--dur", "5.2"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	d, err := db.Open(logDir, "test-eng")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer d.Close()

	entries, err := d.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no entries in database")
	}

	entry := entries[0]
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

	t.Setenv("HOME", tmpDir)

	rootCmd.SetArgs([]string{"log", "--cmd", "vim foo.txt"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	dbPath := filepath.Join(logDir, "test-eng.db")
	if _, err := os.Stat(dbPath); err == nil {
		// DB was created, check it has zero entries
		d, err := db.Open(logDir, "test-eng")
		if err != nil {
			t.Fatalf("failed to open db: %v", err)
		}
		defer d.Close()

		count, err := d.Count()
		if err != nil {
			t.Fatalf("Count failed: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 entries for unmatched tool, got %d", count)
		}
	}
	// If .db doesn't exist at all, that's also correct — unmatched tool shouldn't create it
}
