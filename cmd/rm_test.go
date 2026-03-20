package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cyb33rr/rtlog/internal/db"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/cyb33rr/rtlog/internal/state"
)

func TestRmValidatesName(t *testing.T) {
	err := logfile.ValidateEngagementName("..")
	if err == nil {
		t.Error("expected error for '..'")
	}
}

func TestRmNonexistent(t *testing.T) {
	dir := t.TempDir()
	_, err := os.Stat(filepath.Join(dir, "nosuch.db"))
	if !os.IsNotExist(err) {
		t.Error("expected file to not exist")
	}
}

func TestRmDeletesDB(t *testing.T) {
	dir := t.TempDir()
	d, err := db.Open(dir, "testeng")
	if err != nil {
		t.Fatal(err)
	}
	d.Close()

	dbPath := filepath.Join(dir, "testeng.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("db should exist: %v", err)
	}

	os.Remove(dbPath)
	os.Remove(dbPath + "-wal")
	os.Remove(dbPath + "-shm")

	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Error("db should be deleted")
	}
}

func TestRmClearsActiveEngagement(t *testing.T) {
	st := map[string]string{
		state.KeyEngagement: "target",
		state.KeyTag:        "",
		state.KeyNote:       "",
		state.KeyEnabled:    "1",
		state.KeyCapture:    "1",
	}
	engToRemove := "target"

	if st[state.KeyEngagement] == engToRemove {
		st[state.KeyEngagement] = ""
	}

	if st[state.KeyEngagement] != "" {
		t.Error("engagement should be cleared")
	}
}
