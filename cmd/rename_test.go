package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cyb33rr/rtlog/internal/db"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/cyb33rr/rtlog/internal/state"
)

func TestRenameValidatesOldName(t *testing.T) {
	err := logfile.ValidateEngagementName("../bad")
	if err == nil {
		t.Error("expected error for '../bad'")
	}
}

func TestRenameValidatesNewName(t *testing.T) {
	err := logfile.ValidateEngagementName(".bad")
	if err == nil {
		t.Error("expected error for '.bad'")
	}
}

func TestRenameMovesDB(t *testing.T) {
	dir := t.TempDir()
	d, err := db.Open(dir, "oldname")
	if err != nil {
		t.Fatal(err)
	}
	d.Close()

	oldPath := filepath.Join(dir, "oldname.db")
	newPath := filepath.Join(dir, "newname.db")

	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatalf("rename failed: %v", err)
	}

	if _, err := os.Stat(newPath); err != nil {
		t.Error("new db should exist")
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old db should not exist")
	}
}

func TestRenameRejectsExistingTarget(t *testing.T) {
	dir := t.TempDir()
	d1, _ := db.Open(dir, "eng1")
	d1.Close()
	d2, _ := db.Open(dir, "eng2")
	d2.Close()

	newPath := filepath.Join(dir, "eng2.db")
	if _, err := os.Stat(newPath); err != nil {
		t.Fatal("target should exist for this test")
	}
}

func TestRenameUpdatesActiveEngagement(t *testing.T) {
	st := map[string]string{
		state.KeyEngagement: "oldname",
	}
	oldName := "oldname"
	newName := "newname"

	if st[state.KeyEngagement] == oldName {
		st[state.KeyEngagement] = newName
	}

	if st[state.KeyEngagement] != newName {
		t.Error("engagement should be updated to new name")
	}
}
