# RTLog Red Team Gap Analysis Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix config mismatches, add missing tools, add JSONL export, and add entry-level delete/edit commands.

**Architecture:** Config file changes (tasks 1-2) are pure text edits. JSONL export follows the existing export pattern (function in `internal/export/`, wired in `cmd/export.go`). Delete/edit add new DB methods to `internal/db/db.go` and new Cobra commands in `cmd/`.

**Tech Stack:** Go 1.24, Cobra CLI, SQLite (modernc.org/sqlite), encoding/json

**Spec:** `docs/superpowers/specs/2026-03-17-red-team-gap-analysis-design.md`

---

### Task 1: Update tools.conf and extract.conf

**Files:**
- Modify: `tools.conf`
- Modify: `extract.conf`

This task covers spec sections 1, 2, and 3: fix the mismatch, add new tools, remove non-operational tools.

- [ ] **Step 1: Add 7 missing tools to tools.conf**

Add these tools to the sections indicated:

Under `# Reconnaissance`, after `enum4linux`:
```
enum4linux-ng
```

Under `# Web`, after `wget`:
```
nuclei
httpx
testssl.sh
```

Under `# Remote Access / File Transfer`, after `winexe`:
```
plink
```

Under `# Network Utilities`, remove `dig` and `socat`, add `smbmap`:
```
smbmap
```

The section should become:
```
# Network Utilities
nc
ncat
ldapsearch
snmpwalk
smbmap
```

Under `# Active Directory`, after `responder`:
```
ldapdomaindump
ldeep
windapsearch
adidnsdump
```

Under `# Reconnaissance`, after `enum4linux-ng`:
```
subfinder
amass
```

Under `# Post-Exploitation`, after `sshuttle`:
```
ligolo-ng
ligolo
bore
gost
```

Remove `searchsploit` from the `# Reconnaissance` section.

- [ ] **Step 2: Remove orphaned entries from extract.conf**

Remove these 3 lines from `extract.conf`:
- Line 51: `socat`
- Line 54: `dig positional`
- Line 57: `searchsploit`

After removal, the "Network Utilities" section should be:
```
# Network Utilities
nc positional
ncat positional
ldapsearch target:-H cred:-D=user,-w=pass
snmpwalk positional
```

And the "Utility" section should be:
```
# Utility (no extraction)
responder
chisel
sshuttle
```

- [ ] **Step 3: Run existing tests to ensure no regressions**

Run: `cd /home/cyb3r/RTLog && go test ./...`
Expected: All tests pass (config changes don't affect code behavior, but verify nothing broke).

- [ ] **Step 4: Commit**

```bash
git add tools.conf extract.conf
git commit -m "feat: update tracked tools for red team ops

Add 16 tools: enum4linux-ng, smbmap, plink, ldapdomaindump, ldeep,
windapsearch, adidnsdump, nuclei, httpx, testssl.sh, subfinder,
amass, ligolo-ng, ligolo, bore, gost.

Remove 3 non-operational: searchsploit, dig, socat.
Clean up orphaned extract.conf entries."
```

---

### Task 2: Add JSONL export function

**Files:**
- Create: `internal/export/jsonl.go`
- Test: `internal/export/jsonl_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/export/jsonl_test.go`:

```go
package export

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cyb33rr/rtlog/internal/logfile"
)

func TestExportJSONL_SingleEntry(t *testing.T) {
	entries := []logfile.LogEntry{
		{
			ID: 1, Ts: "2026-03-17T14:30:00Z", Epoch: 1742222400,
			User: "cyb3r", Host: "kali", TTY: "pts/0", Cwd: "/tmp",
			Tool: "nmap", Cmd: "nmap -sV 10.0.0.1", Exit: 0, Dur: 5.2,
			Tag: "recon", Note: "test", Out: "",
		},
	}

	result := ExportJSONL(entries)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Verify id is present (LogEntry has json:"-" on ID, so export must handle this)
	if id, ok := parsed["id"]; !ok {
		t.Error("missing 'id' field in JSONL output")
	} else if id.(float64) != 1 {
		t.Errorf("id = %v, want 1", id)
	}

	if parsed["tool"] != "nmap" {
		t.Errorf("tool = %v, want nmap", parsed["tool"])
	}
	if parsed["cmd"] != "nmap -sV 10.0.0.1" {
		t.Errorf("cmd = %v, want 'nmap -sV 10.0.0.1'", parsed["cmd"])
	}
}

func TestExportJSONL_MultipleEntries(t *testing.T) {
	entries := []logfile.LogEntry{
		{ID: 1, Ts: "2026-03-17T14:30:00Z", Epoch: 1742222400, User: "a", Host: "h", Cwd: "/", Tool: "nmap", Cmd: "nmap 10.0.0.1"},
		{ID: 2, Ts: "2026-03-17T14:31:00Z", Epoch: 1742222460, User: "a", Host: "h", Cwd: "/", Tool: "curl", Cmd: "curl http://10.0.0.1"},
	}

	result := ExportJSONL(entries)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}

	// Each line must be valid JSON
	for i, line := range lines {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i, err)
		}
	}
}

func TestExportJSONL_Empty(t *testing.T) {
	result := ExportJSONL([]logfile.LogEntry{})
	if result != "" {
		t.Errorf("expected empty string for empty entries, got %q", result)
	}
}

func TestExportJSONL_SpecialChars(t *testing.T) {
	entries := []logfile.LogEntry{
		{ID: 1, Ts: "2026-03-17T14:30:00Z", Epoch: 1742222400, User: "a", Host: "h", Cwd: "/", Tool: "curl",
			Cmd: `curl -H "Authorization: Bearer token" http://10.0.0.1`, Note: "line1\nline2"},
	}

	result := ExportJSONL(entries)
	lines := strings.Split(strings.TrimSpace(result), "\n")

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("invalid JSON with special chars: %v", err)
	}
	if parsed["note"] != "line1\nline2" {
		t.Errorf("note not preserved: %v", parsed["note"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/cyb3r/RTLog && go test ./internal/export/ -run TestExportJSONL -v`
Expected: FAIL — `ExportJSONL` not defined.

- [ ] **Step 3: Write the implementation**

Create `internal/export/jsonl.go`:

```go
package export

import (
	"encoding/json"
	"strings"

	"github.com/cyb33rr/rtlog/internal/logfile"
)

// jsonlEntry is a local struct that includes the ID field for JSONL export.
// LogEntry.ID has `json:"-"` which would omit it from encoding/json output.
type jsonlEntry struct {
	ID    int64   `json:"id"`
	Ts    string  `json:"ts"`
	Epoch int64   `json:"epoch"`
	User  string  `json:"user"`
	Host  string  `json:"host"`
	TTY   string  `json:"tty"`
	Cwd   string  `json:"cwd"`
	Tool  string  `json:"tool"`
	Cmd   string  `json:"cmd"`
	Exit  int     `json:"exit"`
	Dur   float64 `json:"dur"`
	Tag   string  `json:"tag"`
	Note  string  `json:"note"`
	Out   string  `json:"out"`
}

func toJSONLEntry(e logfile.LogEntry) jsonlEntry {
	return jsonlEntry{
		ID: e.ID, Ts: e.Ts, Epoch: e.Epoch, User: e.User, Host: e.Host,
		TTY: e.TTY, Cwd: e.Cwd, Tool: e.Tool, Cmd: e.Cmd, Exit: e.Exit,
		Dur: e.Dur, Tag: e.Tag, Note: e.Note, Out: e.Out,
	}
}

// ExportJSONL renders entries as newline-delimited JSON (one object per line).
func ExportJSONL(entries []logfile.LogEntry) string {
	if len(entries) == 0 {
		return ""
	}

	var b strings.Builder
	for _, e := range entries {
		data, err := json.Marshal(toJSONLEntry(e))
		if err != nil {
			continue
		}
		b.Write(data)
		b.WriteByte('\n')
	}

	return strings.TrimRight(b.String(), "\n")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/cyb3r/RTLog && go test ./internal/export/ -run TestExportJSONL -v`
Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/export/jsonl.go internal/export/jsonl_test.go
git commit -m "feat: add JSONL export function with tests"
```

---

### Task 3: Wire JSONL export into CLI

**Files:**
- Modify: `cmd/export.go`

- [ ] **Step 1: Update export command to accept jsonl format**

In `cmd/export.go`, make these changes:

1. Update `Use` string from `"export <md|csv>"` to `"export <md|csv|jsonl>"`
2. Update `Short` from `"Export entries as Markdown or CSV"` to `"Export entries as Markdown, CSV, or JSONL"`
3. Update `Long` from `"Export log entries to a Markdown table or CSV file."` to `"Export log entries to a Markdown table, CSV, or JSONL file."`
4. Update the format validation from `format != "md" && format != "csv"` to `format != "md" && format != "csv" && format != "jsonl"`
5. Update the error message from `"use md or csv"` to `"use md, csv, or jsonl"`
6. Add the `jsonl` case to the switch:
```go
	case "jsonl":
		text = export.ExportJSONL(entries)
```

- [ ] **Step 2: Build and verify**

Run: `cd /home/cyb3r/RTLog && go build .`
Expected: Successful build, no errors.

- [ ] **Step 3: Run all tests**

Run: `cd /home/cyb3r/RTLog && go test ./...`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
git add cmd/export.go
git commit -m "feat: wire JSONL format into export command"
```

---

### Task 4: Add GetByID and Delete DB methods

**Files:**
- Modify: `internal/db/db.go`
- Modify: `internal/db/db_test.go`

- [ ] **Step 1: Write failing tests for GetByID**

Append to `internal/db/db_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/cyb3r/RTLog && go test ./internal/db/ -run TestGetByID -v`
Expected: FAIL — `GetByID` not defined.

- [ ] **Step 3: Implement GetByID**

Add to `internal/db/db.go`, before the `Close` method:

```go
// GetByID returns a single entry by ID, or nil if not found.
func (d *DB) GetByID(id int64) (*logfile.LogEntry, error) {
	row := d.db.QueryRow(
		"SELECT id, ts, epoch, user, host, tty, cwd, tool, cmd, exit, dur, tag, note, out FROM entries WHERE id = ?", id)
	var e logfile.LogEntry
	err := row.Scan(&e.ID, &e.Ts, &e.Epoch, &e.User, &e.Host, &e.TTY,
		&e.Cwd, &e.Tool, &e.Cmd, &e.Exit, &e.Dur, &e.Tag, &e.Note, &e.Out)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get entry by id: %w", err)
	}
	return &e, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/cyb3r/RTLog && go test ./internal/db/ -run TestGetByID -v`
Expected: PASS.

- [ ] **Step 5: Write failing test for Delete**

Append to `internal/db/db_test.go`:

```go
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
```

- [ ] **Step 6: Run test to verify it fails**

Run: `cd /home/cyb3r/RTLog && go test ./internal/db/ -run TestDelete -v`
Expected: FAIL — `Delete` not defined.

- [ ] **Step 7: Implement Delete**

Add to `internal/db/db.go`, after the `GetByID` method:

```go
// Delete removes a single entry by ID.
func (d *DB) Delete(id int64) error {
	_, err := d.db.Exec("DELETE FROM entries WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete entry: %w", err)
	}
	return nil
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `cd /home/cyb3r/RTLog && go test ./internal/db/ -run TestDelete -v`
Expected: Both TestDelete and TestDeleteNonexistent PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go
git commit -m "feat: add GetByID and Delete DB methods"
```

---

### Task 5: Add Update DB method

**Files:**
- Modify: `internal/db/db.go`
- Modify: `internal/db/db_test.go`

- [ ] **Step 1: Write failing tests for Update**

Append to `internal/db/db_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/cyb3r/RTLog && go test ./internal/db/ -run TestUpdate -v`
Expected: FAIL — `Update` not defined.

- [ ] **Step 3: Implement Update**

Add to `internal/db/db.go`, after the `Delete` method:

```go
// allowedUpdateColumns is the whitelist of columns that can be modified via Update.
var allowedUpdateColumns = map[string]bool{
	"tag":  true,
	"note": true,
}

// Update modifies allowed fields on a single entry by ID.
// Only "tag" and "note" columns are permitted; other column names return an error.
func (d *DB) Update(id int64, fields map[string]string) error {
	if len(fields) == 0 {
		return fmt.Errorf("no fields to update")
	}

	var setClauses []string
	var args []interface{}
	for col, val := range fields {
		if !allowedUpdateColumns[col] {
			return fmt.Errorf("column %q is not editable", col)
		}
		setClauses = append(setClauses, col+" = ?")
		args = append(args, val)
	}
	args = append(args, id)

	query := "UPDATE entries SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
	_, err := d.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update entry: %w", err)
	}
	return nil
}
```

Also add `"strings"` to the import block in `db.go` if not already present.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/cyb3r/RTLog && go test ./internal/db/ -run TestUpdate -v`
Expected: All 4 Update tests PASS.

- [ ] **Step 5: Run all DB tests**

Run: `cd /home/cyb3r/RTLog && go test ./internal/db/ -v`
Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go
git commit -m "feat: add Update DB method for tag/note editing"
```

---

### Task 6: Add delete CLI command

**Files:**
- Create: `cmd/delete.go`

- [ ] **Step 1: Create cmd/delete.go**

```go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/spf13/cobra"

	"golang.org/x/term"
)

var deleteYes bool

var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a single log entry by ID",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid id %q\n", args[0])
			os.Exit(1)
		}

		d, err := openEngagementDB()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer d.Close()

		entry, err := d.GetByID(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if entry == nil {
			fmt.Fprintf(os.Stderr, "error: entry %d not found\n", id)
			os.Exit(1)
		}

		// Show the entry
		m := logfile.ToMap(*entry)
		fmt.Println(display.FmtEntry(m, int(entry.ID), 1))

		if !deleteYes {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				fmt.Fprintln(os.Stderr, "Not a terminal. Use -y to confirm.")
				os.Exit(1)
			}
			fmt.Printf("Delete entry %d? [y/N] ", id)
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(answer)) != "y" {
				fmt.Println("Aborted.")
				return
			}
		}

		if err := d.Delete(id); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Deleted entry %d.\n", id)
	},
}

func init() {
	deleteCmd.Flags().BoolVarP(&deleteYes, "yes", "y", false, "skip confirmation prompt")
	rootCmd.AddCommand(deleteCmd)
}
```

- [ ] **Step 2: Build and verify**

Run: `cd /home/cyb3r/RTLog && go build .`
Expected: Successful build.

- [ ] **Step 3: Verify help text**

Run: `cd /home/cyb3r/RTLog && go run . delete --help`
Expected: Shows usage with `<id>` argument and `--yes/-y` flag.

- [ ] **Step 4: Run all tests**

Run: `cd /home/cyb3r/RTLog && go test ./...`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/delete.go
git commit -m "feat: add rtlog delete command for single entry removal"
```

---

### Task 7: Add edit CLI command

**Files:**
- Create: `cmd/edit.go`

- [ ] **Step 1: Create cmd/edit.go**

```go
package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/spf13/cobra"
)

var editNote string
var editTag string

var editCmd = &cobra.Command{
	Use:   "edit <id> [--note TEXT] [--tag TEXT]",
	Short: "Edit note or tag on a log entry",
	Long:  "Modify the note or tag on an existing log entry. Other fields are immutable.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid id %q\n", args[0])
			os.Exit(1)
		}

		noteChanged := cmd.Flags().Changed("note")
		tagChanged := cmd.Flags().Changed("tag")

		if !noteChanged && !tagChanged {
			fmt.Fprintln(os.Stderr, "error: at least one of --note or --tag must be provided")
			cmd.Usage()
			os.Exit(1)
		}

		d, err := openEngagementDB()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer d.Close()

		entry, err := d.GetByID(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if entry == nil {
			fmt.Fprintf(os.Stderr, "error: entry %d not found\n", id)
			os.Exit(1)
		}

		fields := map[string]string{}
		if noteChanged {
			fields["note"] = editNote
		}
		if tagChanged {
			fields["tag"] = editTag
		}

		if err := d.Update(id, fields); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		// Show updated entry
		updated, err := d.GetByID(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		m := logfile.ToMap(*updated)
		fmt.Println(display.FmtEntry(m, int(updated.ID), 1))
	},
}

func init() {
	editCmd.Flags().StringVar(&editNote, "note", "", "new note text (empty to clear)")
	editCmd.Flags().StringVar(&editTag, "tag", "", "new tag text (empty to clear)")
	rootCmd.AddCommand(editCmd)
}
```

Key detail: uses `cmd.Flags().Changed("note")` to distinguish between "flag not provided" and "flag provided with empty string" — this lets `--tag ""` clear a tag.

- [ ] **Step 2: Build and verify**

Run: `cd /home/cyb3r/RTLog && go build .`
Expected: Successful build.

- [ ] **Step 3: Verify help text**

Run: `cd /home/cyb3r/RTLog && go run . edit --help`
Expected: Shows usage with `<id>` argument and `--note`/`--tag` flags.

- [ ] **Step 4: Run all tests**

Run: `cd /home/cyb3r/RTLog && go test ./...`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/edit.go
git commit -m "feat: add rtlog edit command for note/tag modification"
```

---

### Task 8: Final verification

- [ ] **Step 1: Full test suite**

Run: `cd /home/cyb3r/RTLog && go test ./... -v`
Expected: All tests pass across all packages.

- [ ] **Step 2: Build binary**

Run: `cd /home/cyb3r/RTLog && go build -o rtlog .`
Expected: Clean build, no warnings.

- [ ] **Step 3: Verify all new commands appear in help**

Run: `cd /home/cyb3r/RTLog && ./rtlog --help`
Expected: `delete`, `edit` appear in command list. `export` help shows `md|csv|jsonl`.

- [ ] **Step 4: Spot-check tools.conf**

Run: `cd /home/cyb3r/RTLog && grep -c '^[a-z]' tools.conf`
Expected: Count reflects additions (+16) minus removals (-3) from original (42). Should be 55 non-comment, non-empty lines.
