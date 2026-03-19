package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cyb33rr/rtlog/internal/logfile"
)

// sampleEntry returns a LogEntry with reasonable defaults.
func sampleEntry(i int) logfile.LogEntry {
	return logfile.LogEntry{
		Ts:    fmt.Sprintf("2025-01-15T10:%02d:00Z", i),
		Epoch: 1705312800 + int64(i*60),
		User:  "operator",
		Host:  "kali",
		TTY:   "pts/0",
		Cwd:   "/tmp",
		Tool:  "nmap",
		Cmd:   fmt.Sprintf("nmap -sV 10.0.0.%d", i),
		Exit:  0,
		Dur:   1.5,
		Tag:   "recon",
		Note:  "",
		Out:   "",
	}
}

// insertN inserts n entries into the database using sampleEntry.
func insertN(t *testing.T, d *DB, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := d.Insert(sampleEntry(i)); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	d, err := Open(dir, "test")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestOpenCreatesDB(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir, "myop")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	dbPath := filepath.Join(dir, "myop.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("expected %s to exist", dbPath)
	}
}

func TestOpenSetsWAL(t *testing.T) {
	d := openTestDB(t)

	var mode string
	if err := d.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want %q", mode, "wal")
	}
}

func TestOpenSetsUserVersion(t *testing.T) {
	d := openTestDB(t)

	var version int
	if err := d.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("query user_version: %v", err)
	}
	if version != schemaVersion {
		t.Errorf("user_version = %d, want %d", version, schemaVersion)
	}
}

func TestInsertAndLoadAll(t *testing.T) {
	d := openTestDB(t)

	e := sampleEntry(0)
	if err := d.Insert(e); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	entries, err := d.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}

	got := entries[0]
	if got.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if got.Ts != e.Ts {
		t.Errorf("Ts = %q, want %q", got.Ts, e.Ts)
	}
	if got.Epoch != e.Epoch {
		t.Errorf("Epoch = %d, want %d", got.Epoch, e.Epoch)
	}
	if got.User != e.User {
		t.Errorf("User = %q, want %q", got.User, e.User)
	}
	if got.Host != e.Host {
		t.Errorf("Host = %q, want %q", got.Host, e.Host)
	}
	if got.TTY != e.TTY {
		t.Errorf("TTY = %q, want %q", got.TTY, e.TTY)
	}
	if got.Cwd != e.Cwd {
		t.Errorf("Cwd = %q, want %q", got.Cwd, e.Cwd)
	}
	if got.Tool != e.Tool {
		t.Errorf("Tool = %q, want %q", got.Tool, e.Tool)
	}
	if got.Cmd != e.Cmd {
		t.Errorf("Cmd = %q, want %q", got.Cmd, e.Cmd)
	}
	if got.Exit != e.Exit {
		t.Errorf("Exit = %d, want %d", got.Exit, e.Exit)
	}
	if got.Dur != e.Dur {
		t.Errorf("Dur = %f, want %f", got.Dur, e.Dur)
	}
	if got.Tag != e.Tag {
		t.Errorf("Tag = %q, want %q", got.Tag, e.Tag)
	}
}

func TestLoadByDate(t *testing.T) {
	d := openTestDB(t)

	// Insert entries on two different dates.
	e1 := sampleEntry(0)
	e1.Ts = "2025-01-15T10:00:00Z"
	e2 := sampleEntry(1)
	e2.Ts = "2025-01-16T11:00:00Z"
	e3 := sampleEntry(2)
	e3.Ts = "2025-01-15T12:00:00Z"

	for _, e := range []logfile.LogEntry{e1, e2, e3} {
		if err := d.Insert(e); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	entries, err := d.LoadByDate("2025-01-15")
	if err != nil {
		t.Fatalf("LoadByDate: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	entries2, err := d.LoadByDate("2025-01-16")
	if err != nil {
		t.Fatalf("LoadByDate: %v", err)
	}
	if len(entries2) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries2))
	}

	entries3, err := d.LoadByDate("2025-01-17")
	if err != nil {
		t.Fatalf("LoadByDate: %v", err)
	}
	if len(entries3) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries3))
	}
}

func TestSearch(t *testing.T) {
	d := openTestDB(t)

	e1 := sampleEntry(0)
	e1.Tool = "nmap"
	e1.Cmd = "nmap -sV 10.0.0.1"

	e2 := sampleEntry(1)
	e2.Tool = "gobuster"
	e2.Cmd = "gobuster dir -u http://target"

	e3 := sampleEntry(2)
	e3.Tool = "nmap"
	e3.Cmd = "nmap -p- 10.0.0.2"

	for _, e := range []logfile.LogEntry{e1, e2, e3} {
		if err := d.Insert(e); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	// Search by tool name.
	entries, err := d.Search("gobuster")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries for 'gobuster', want 1", len(entries))
	}
	if entries[0].Tool != "gobuster" {
		t.Errorf("Tool = %q, want %q", entries[0].Tool, "gobuster")
	}

	// Search by command content.
	entries2, err := d.Search("nmap")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(entries2) != 2 {
		t.Fatalf("got %d entries for 'nmap', want 2", len(entries2))
	}
}

func TestSearchDoesNotMatchUserHost(t *testing.T) {
	d := openTestDB(t)

	e1 := sampleEntry(0)
	e1.User = "alice"
	e1.Host = "specialhost"
	e2 := sampleEntry(1)
	e2.User = "bob"

	for _, e := range []logfile.LogEntry{e1, e2} {
		if err := d.Insert(e); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	// Search by user should NOT match (user field excluded).
	entries, err := d.Search("alice")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d entries for 'alice', want 0 (user field not searched)", len(entries))
	}

	// Search by host should NOT match (host field excluded).
	entries, err = d.Search("specialhost")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d entries for 'specialhost', want 0 (host field not searched)", len(entries))
	}
}

func TestCount(t *testing.T) {
	d := openTestDB(t)
	insertN(t, d, 7)

	count, err := d.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 7 {
		t.Errorf("Count = %d, want 7", count)
	}
}

func TestClear(t *testing.T) {
	d := openTestDB(t)
	insertN(t, d, 5)

	if err := d.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	count, err := d.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Errorf("Count after Clear = %d, want 0", count)
	}
}

func TestGetByID(t *testing.T) {
	d := openTestDB(t)
	insertN(t, d, 3)

	// Get existing entry
	entry, err := d.GetByID(2)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if entry == nil {
		t.Fatal("GetByID returned nil for existing entry")
	}
	if entry.ID != 2 {
		t.Errorf("ID = %d, want 2", entry.ID)
	}

	// Get nonexistent entry
	entry, err = d.GetByID(999)
	if err != nil {
		t.Fatalf("GetByID for missing: %v", err)
	}
	if entry != nil {
		t.Errorf("expected nil for nonexistent ID, got %+v", entry)
	}
}

func TestDelete(t *testing.T) {
	d := openTestDB(t)
	insertN(t, d, 3)

	// Delete entry 2
	if err := d.Delete(2); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Count should be 2
	count, err := d.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("Count after delete = %d, want 2", count)
	}

	// Entry 2 should be gone
	entry, err := d.GetByID(2)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if entry != nil {
		t.Error("entry 2 still exists after delete")
	}

	// Entries 1 and 3 should still exist
	for _, id := range []int64{1, 3} {
		e, err := d.GetByID(id)
		if err != nil {
			t.Fatalf("GetByID(%d): %v", id, err)
		}
		if e == nil {
			t.Errorf("entry %d missing after deleting entry 2", id)
		}
	}
}

func TestDeleteNonexistent(t *testing.T) {
	d := openTestDB(t)
	insertN(t, d, 1)

	// Deleting nonexistent ID should not error
	if err := d.Delete(999); err != nil {
		t.Fatalf("Delete nonexistent: %v", err)
	}

	// Count unchanged
	count, err := d.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}
}

func TestUpdate(t *testing.T) {
	d := openTestDB(t)
	insertN(t, d, 1)

	// Update tag
	if err := d.Update(1, map[string]string{"tag": "privesc"}); err != nil {
		t.Fatalf("Update tag: %v", err)
	}
	e, _ := d.GetByID(1)
	if e.Tag != "privesc" {
		t.Errorf("Tag = %q, want %q", e.Tag, "privesc")
	}

	// Update note
	if err := d.Update(1, map[string]string{"note": "escalated via potato"}); err != nil {
		t.Fatalf("Update note: %v", err)
	}
	e, _ = d.GetByID(1)
	if e.Note != "escalated via potato" {
		t.Errorf("Note = %q, want %q", e.Note, "escalated via potato")
	}

	// Update both
	if err := d.Update(1, map[string]string{"tag": "exfil", "note": "done"}); err != nil {
		t.Fatalf("Update both: %v", err)
	}
	e, _ = d.GetByID(1)
	if e.Tag != "exfil" || e.Note != "done" {
		t.Errorf("got tag=%q note=%q, want tag=%q note=%q", e.Tag, e.Note, "exfil", "done")
	}
}

func TestUpdateClearField(t *testing.T) {
	d := openTestDB(t)

	e := sampleEntry(0)
	e.Tag = "recon"
	e.Note = "initial"
	d.Insert(e)

	// Clear tag with empty string
	if err := d.Update(1, map[string]string{"tag": ""}); err != nil {
		t.Fatalf("Update clear tag: %v", err)
	}
	got, _ := d.GetByID(1)
	if got.Tag != "" {
		t.Errorf("Tag = %q after clear, want empty", got.Tag)
	}
	// Note should be unchanged
	if got.Note != "initial" {
		t.Errorf("Note = %q, want %q (unchanged)", got.Note, "initial")
	}
}

func TestUpdateInvalidColumn(t *testing.T) {
	d := openTestDB(t)
	insertN(t, d, 1)

	// Attempting to update an immutable field should error
	err := d.Update(1, map[string]string{"cmd": "malicious"})
	if err == nil {
		t.Error("expected error when updating immutable field 'cmd'")
	}
}

func TestUpdateNonexistentID(t *testing.T) {
	d := openTestDB(t)

	// Update on nonexistent entry should not error (no rows affected is fine)
	err := d.Update(999, map[string]string{"tag": "test"})
	if err != nil {
		t.Fatalf("Update nonexistent: %v", err)
	}
}

func TestReopenExistingDB(t *testing.T) {
	dir := t.TempDir()

	// Open, insert, close.
	d1, err := Open(dir, "persist")
	if err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	e := sampleEntry(0)
	if err := d1.Insert(e); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := d1.Close(); err != nil {
		t.Fatalf("Close 1: %v", err)
	}

	// Reopen and verify entry persisted.
	d2, err := Open(dir, "persist")
	if err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	defer d2.Close()

	entries, err := d2.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries after reopen, want 1", len(entries))
	}
	if entries[0].Cmd != e.Cmd {
		t.Errorf("Cmd = %q, want %q", entries[0].Cmd, e.Cmd)
	}
	if entries[0].ID == 0 {
		t.Error("expected non-zero ID after reopen")
	}
}

func TestSearchByDate(t *testing.T) {
	d := openTestDB(t)

	e1 := sampleEntry(0)
	e1.Ts = "2025-01-15T10:00:00Z"
	e1.Tool = "nmap"
	e1.Cmd = "nmap -sV 10.0.0.1"

	e2 := sampleEntry(1)
	e2.Ts = "2025-01-16T11:00:00Z"
	e2.Tool = "nmap"
	e2.Cmd = "nmap -p- 10.0.0.2"

	e3 := sampleEntry(2)
	e3.Ts = "2025-01-15T12:00:00Z"
	e3.Tool = "gobuster"
	e3.Cmd = "gobuster dir -u http://target"

	for _, e := range []logfile.LogEntry{e1, e2, e3} {
		if err := d.Insert(e); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	// Search "nmap" on 2025-01-15: only e1 matches (e2 is wrong date).
	entries, err := d.SearchByDate("nmap", "2025-01-15")
	if err != nil {
		t.Fatalf("SearchByDate: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Cmd != "nmap -sV 10.0.0.1" {
		t.Errorf("Cmd = %q, want %q", entries[0].Cmd, "nmap -sV 10.0.0.1")
	}

	// Search "nmap" on 2025-01-16: only e2 matches.
	entries2, err := d.SearchByDate("nmap", "2025-01-16")
	if err != nil {
		t.Fatalf("SearchByDate: %v", err)
	}
	if len(entries2) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries2))
	}

	// Search "gobuster" on 2025-01-16: no matches (gobuster is on 01-15).
	entries3, err := d.SearchByDate("gobuster", "2025-01-16")
	if err != nil {
		t.Fatalf("SearchByDate: %v", err)
	}
	if len(entries3) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries3))
	}
}
