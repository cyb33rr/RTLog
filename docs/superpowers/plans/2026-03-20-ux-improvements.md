# RTLog UX Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add engagement rm/rename commands, TUI interactive delete/edit, and remove the standalone edit command.

**Architecture:** Three independent features sharing the existing `cmd/` + `internal/` structure. Engagement commands follow the `new`/`switch` pattern. TUI changes use a callback-based state machine in the Selector to keep `display` decoupled from `db`. The standalone `edit` command is deleted after TUI edit is working.

**Tech Stack:** Go, cobra, SQLite (modernc.org/sqlite), golang.org/x/term

**Spec:** `docs/superpowers/specs/2026-03-20-ux-improvements-design.md`

---

### Task 1: Add `rtlog rm` command

**Files:**
- Create: `cmd/rm.go`
- Create: `cmd/rm_test.go`

- [ ] **Step 1: Write the test file**

```go
// cmd/rm_test.go
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
	// ".." is rejected by ValidateEngagementName
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

	// Simulate removal
	os.Remove(dbPath)
	os.Remove(dbPath + "-wal")
	os.Remove(dbPath + "-shm")

	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Error("db should be deleted")
	}
}

func TestRmClearsActiveEngagement(t *testing.T) {
	// If removing the active engagement, state.engagement should be cleared
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
```

- [ ] **Step 2: Run tests to verify they pass (these test primitives, not the command itself yet)**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -run TestRm -v`
Expected: All PASS

- [ ] **Step 3: Write the rm command**

```go
// cmd/rm.go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/db"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/cyb33rr/rtlog/internal/state"

	"golang.org/x/term"
)

var rmYes bool

var rmCmd = &cobra.Command{
	Use:   "rm <engagement>",
	Short: "Delete an engagement and its database",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		if err := logfile.ValidateEngagementName(name); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid engagement name: %v\n", err)
			os.Exit(1)
		}

		dir := logfile.LogDir()
		dbPath := filepath.Join(dir, name+".db")

		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "error: engagement %q not found\n", name)
			os.Exit(1)
		}

		// Open DB to get count and force WAL checkpoint, then close
		d, err := db.Open(dir, name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		count, err := d.Count()
		if err != nil {
			d.Close()
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		d.Close()

		if !rmYes {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				fmt.Fprintln(os.Stderr, "Not a terminal. Use -y to confirm.")
				os.Exit(1)
			}
			fmt.Printf("Delete engagement %q (%d entries)? [y/N] ", name, count)
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(answer)) != "y" {
				fmt.Println("Aborted.")
				return
			}
		}

		// Remove DB and WAL sidecar files
		if err := os.Remove(dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")

		// Clear engagement from state if it was active
		st := state.ReadState()
		if st[state.KeyEngagement] == name {
			if _, err := state.UpdateState(map[string]string{state.KeyEngagement: ""}); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
				os.Exit(1)
			}
		}

		fmt.Printf("Deleted engagement %q (%d entries).\n", name, count)
	},
}

func init() {
	rmCmd.Flags().BoolVarP(&rmYes, "yes", "y", false, "skip confirmation prompt")
	rootCmd.AddCommand(rmCmd)
}
```

- [ ] **Step 4: Run all tests**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -run TestRm -v`
Expected: All PASS

- [ ] **Step 5: Build and smoke test**

Run: `cd /home/cyb3r/RTLog && go build -o rtlog . && ./rtlog rm --help`
Expected: Shows usage `rm <engagement>` with `-y/--yes` flag

- [ ] **Step 6: Commit**

```bash
git add cmd/rm.go cmd/rm_test.go
git commit -m "feat(cmd): add rtlog rm command for engagement deletion"
```

---

### Task 2: Add `rtlog rename` command

**Files:**
- Create: `cmd/rename.go`
- Create: `cmd/rename_test.go`

- [ ] **Step 1: Write the test file**

```go
// cmd/rename_test.go
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
	// In the real command, this stat check causes an error before rename
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
```

- [ ] **Step 2: Run tests**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -run TestRename -v`
Expected: All PASS

- [ ] **Step 3: Write the rename command**

```go
// cmd/rename.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/db"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/cyb33rr/rtlog/internal/state"
)

var renameCmd = &cobra.Command{
	Use:   "rename <old> <new>",
	Short: "Rename an engagement",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		oldName := args[0]
		newName := args[1]

		if err := logfile.ValidateEngagementName(oldName); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid engagement name %q: %v\n", oldName, err)
			os.Exit(1)
		}
		if err := logfile.ValidateEngagementName(newName); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid engagement name %q: %v\n", newName, err)
			os.Exit(1)
		}

		dir := logfile.LogDir()
		oldPath := filepath.Join(dir, oldName+".db")
		newPath := filepath.Join(dir, newName+".db")

		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "error: engagement %q not found\n", oldName)
			os.Exit(1)
		}

		if _, err := os.Stat(newPath); err == nil {
			fmt.Fprintf(os.Stderr, "error: engagement %q already exists\n", newName)
			os.Exit(1)
		}

		// Open and close DB to force WAL checkpoint
		d, err := db.Open(dir, oldName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		d.Close()

		// Rename main DB file
		if err := os.Rename(oldPath, newPath); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		// Rename WAL sidecars if they still exist
		os.Rename(oldPath+"-wal", newPath+"-wal")
		os.Rename(oldPath+"-shm", newPath+"-shm")

		// Update state if active engagement was renamed
		st := state.ReadState()
		if st[state.KeyEngagement] == oldName {
			if _, err := state.UpdateState(map[string]string{state.KeyEngagement: newName}); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
				os.Exit(1)
			}
		}

		fmt.Printf("[rtlog] Renamed: %s -> %s\n", oldName, newName)
	},
}

func init() {
	rootCmd.AddCommand(renameCmd)
}
```

- [ ] **Step 4: Run all tests**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -run TestRename -v`
Expected: All PASS

- [ ] **Step 5: Build and smoke test**

Run: `cd /home/cyb3r/RTLog && go build -o rtlog . && ./rtlog rename --help`
Expected: Shows usage `rename <old> <new>`

- [ ] **Step 6: Commit**

```bash
git add cmd/rename.go cmd/rename_test.go
git commit -m "feat(cmd): add rtlog rename command for engagement renaming"
```

---

### Task 3: Add `"id"` to `ToMap()` output

**Files:**
- Modify: `internal/logfile/logfile.go:140-156`

- [ ] **Step 1: Add `"id"` to the ToMap function**

In `internal/logfile/logfile.go`, add `"id": e.ID` to the `ToMap()` return map:

```go
func ToMap(e LogEntry) map[string]interface{} {
	return map[string]interface{}{
		"id":    e.ID,
		"ts":    e.Ts,
		"epoch": e.Epoch,
		"user":  e.User,
		"host":  e.Host,
		"tty":   e.TTY,
		"cwd":   e.Cwd,
		"tool":  e.Tool,
		"cmd":   e.Cmd,
		"exit":  e.Exit,
		"dur":   e.Dur,
		"tag":   e.Tag,
		"note":  e.Note,
		"out":   e.Out,
	}
}
```

- [ ] **Step 2: Run existing tests to ensure nothing breaks**

Run: `cd /home/cyb3r/RTLog && go test ./...`
Expected: All PASS (the `"id"` key is additive — no existing code reads it)

- [ ] **Step 3: Commit**

```bash
git add internal/logfile/logfile.go
git commit -m "feat(logfile): include id in ToMap output for TUI callbacks"
```

---

### Task 4: Add TUI state machine and delete/edit callbacks to Selector

**Files:**
- Modify: `internal/display/selector.go`

This is the largest task. It modifies the Selector to support multiple modes.

- [ ] **Step 1: Add mode types, callback fields, and edit buffer to the Selector struct**

At the top of `selector.go`, after the imports, add the mode enum. Then add fields to the `Selector` struct.

Add after the `import` block (before `ApplyFilters`):

```go
// selectorMode represents the current interaction mode of the Selector.
type selectorMode int

const (
	modeNormal        selectorMode = iota
	modeConfirmDelete              // waiting for y/n
	modeEditChoose                 // waiting for t/n
	modeEditTag                    // editing tag value
	modeEditNote                   // editing note value
)
```

Add these fields to the `Selector` struct (after the existing filtering fields):

```go
	// Mutation callbacks (nil = feature disabled)
	OnDelete func(id int64) error
	OnUpdate func(id int64, fields map[string]string) error

	// Mode state
	mode    selectorMode
	editBuf string // buffer for inline editing in modeEditTag/modeEditNote
```

- [ ] **Step 2: Add helper to get the DB id of the currently selected entry**

Add this method to Selector:

```go
// selectedID returns the database ID of the currently selected entry, or -1.
func (s *Selector) selectedID() int64 {
	if len(s.filtered) == 0 || s.cursor >= len(s.filtered) {
		return -1
	}
	entry := s.entries[s.filtered[s.cursor]]
	id, ok := entry["id"].(int64)
	if !ok {
		return -1
	}
	return id
}
```

- [ ] **Step 3: Add the delete handler method**

```go
// handleDelete processes the Ctrl+D → y/n confirmation flow.
func (s *Selector) handleDelete() {
	id := s.selectedID()
	if id < 0 || s.OnDelete == nil {
		return
	}
	if err := s.OnDelete(id); err != nil {
		s.mode = modeNormal
		return
	}
	// Splice entry out of s.entries
	entryIdx := s.filtered[s.cursor]
	s.entries = append(s.entries[:entryIdx], s.entries[entryIdx+1:]...)
	// Rebuild filtered list and adjust cursor
	s.filtered = ApplyFilters(s.entries, s.filter, s.tagFilter, s.failOnly, s.useRegex)
	s.allTags = CollectTags(s.entries)
	if s.cursor >= len(s.filtered) {
		s.cursor = len(s.filtered) - 1
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
	s.expanded = false
	s.mode = modeNormal
}
```

- [ ] **Step 4: Add the edit save method**

```go
// handleEditSave persists the edit buffer to the database and updates in-memory state.
func (s *Selector) handleEditSave() {
	id := s.selectedID()
	if id < 0 || s.OnUpdate == nil {
		s.mode = modeNormal
		return
	}

	var field string
	if s.mode == modeEditTag {
		field = "tag"
	} else {
		field = "note"
	}

	if err := s.OnUpdate(id, map[string]string{field: s.editBuf}); err != nil {
		s.mode = modeNormal
		return
	}

	// Update in-memory entry
	entryIdx := s.filtered[s.cursor]
	s.entries[entryIdx][field] = s.editBuf

	// Rebuild filters in case the edit affects visible entries
	s.filtered = ApplyFilters(s.entries, s.filter, s.tagFilter, s.failOnly, s.useRegex)
	s.allTags = CollectTags(s.entries)
	if s.cursor >= len(s.filtered) {
		s.cursor = len(s.filtered) - 1
	}
	if s.cursor < 0 {
		s.cursor = 0
	}

	s.mode = modeNormal
}
```

- [ ] **Step 5: Update the `renderFilterBar` method to handle all modes**

Replace the existing `renderFilterBar` method with a version that switches on `s.mode`:

```go
func (s *Selector) renderFilterBar() string {
	switch s.mode {
	case modeConfirmDelete:
		id := s.selectedID()
		return fmt.Sprintf("  Delete entry #%d? (y/n)", id)
	case modeEditChoose:
		return "  Edit: (t)ag or (n)ote?  [Esc cancel]"
	case modeEditTag:
		return fmt.Sprintf("  Tag: %s_  [Enter save, Esc cancel]", s.editBuf)
	case modeEditNote:
		return fmt.Sprintf("  Note: %s_  [Enter save, Esc cancel]", s.editBuf)
	}

	// modeNormal — original filter bar
	var parts []string

	if s.tagFilter != "" {
		parts = append(parts, Colorize(fmt.Sprintf("[%s]", s.tagFilter), Yellow))
	}
	if s.failOnly {
		parts = append(parts, Colorize("[!fail]", Red))
	}

	total := len(s.entries)
	matched := len(s.filtered)
	hasFilter := s.filter != "" || s.tagFilter != "" || s.failOnly
	if hasFilter {
		parts = append(parts, fmt.Sprintf("%d/%d matches", matched, total))
	} else {
		parts = append(parts, fmt.Sprintf("%d entries", total))
	}

	if s.useRegex {
		if s.filter != "" {
			if _, err := regexp.Compile(s.filter); err != nil {
				parts = append(parts, fmt.Sprintf("▸ /%s/_ %s", s.filter, Colorize("[invalid regex]", Red)))
			} else {
				parts = append(parts, fmt.Sprintf("▸ /%s/_ %s", s.filter, Colorize("[regex]", Cyan)))
			}
		} else {
			parts = append(parts, fmt.Sprintf("▸ //_  %s", Colorize("[regex]", Cyan)))
		}
	} else {
		parts = append(parts, fmt.Sprintf("▸ %s_", s.filter))
	}

	return "  " + strings.Join(parts, "   ")
}
```

- [ ] **Step 6: Update the `Run` method to dispatch by mode**

In the `Run()` method, modify the key handling in the `if n == 1` block. The new logic:

- `Esc` (byte 27): if in a non-normal mode, return to `modeNormal`; if in `modeNormal`, quit
- `Ctrl+D` (byte 4): if `modeNormal` and entries exist and `OnDelete != nil`, enter `modeConfirmDelete`
- `Ctrl+E` (byte 5): if `modeNormal` and entries exist and `OnUpdate != nil`, enter `modeEditChoose`
- In `modeConfirmDelete`: `y` calls `handleDelete()`, anything else returns to `modeNormal`
- In `modeEditChoose`: `t` enters `modeEditTag` (loads current tag into editBuf), `n` enters `modeEditNote` (loads current note into editBuf), anything else returns to `modeNormal`
- In `modeEditTag`/`modeEditNote`: `Enter` (13) calls `handleEditSave()`, `Backspace` deletes from editBuf, printable chars append to editBuf

Replace the `if n == 1 {` block in `Run()`:

```go
		if n == 1 {
			switch s.mode {
			case modeConfirmDelete:
				if buf[0] == 'y' || buf[0] == 'Y' {
					s.handleDelete()
				} else {
					s.mode = modeNormal
				}
				continue

			case modeEditChoose:
				switch buf[0] {
				case 't', 'T':
					s.editBuf = getString(s.entries[s.filtered[s.cursor]], "tag", "")
					s.mode = modeEditTag
				case 'n', 'N':
					s.editBuf = getString(s.entries[s.filtered[s.cursor]], "note", "")
					s.mode = modeEditNote
				case 27: // Esc
					s.mode = modeNormal
				default:
					s.mode = modeNormal
				}
				continue

			case modeEditTag, modeEditNote:
				switch buf[0] {
				case 27: // Esc
					s.mode = modeNormal
				case 13: // Enter — save
					s.handleEditSave()
				case 127, 8: // Backspace
					if len(s.editBuf) > 0 {
						runes := []rune(s.editBuf)
						s.editBuf = string(runes[:len(runes)-1])
					}
				default:
					if buf[0] >= 0x20 && buf[0] <= 0x7E {
						s.editBuf += string(buf[0])
					}
				}
				continue
			}

			// modeNormal handlers
			switch buf[0] {
			case 27: // Esc
				return nil
			case 13: // Enter
				if len(s.filtered) > 0 {
					s.expanded = !s.expanded
					s.outScroll = 0
				}
			case 9: // Tab — cycle tag filter
				if len(s.allTags) > 0 {
					s.tagIdx = (s.tagIdx + 1) % (len(s.allTags) + 1)
					if s.tagIdx == 0 {
						s.tagFilter = ""
					} else {
						s.tagFilter = s.allTags[s.tagIdx-1]
					}
					s.applyAndReset()
				}
			case 4: // Ctrl+D — delete
				if len(s.filtered) > 0 && s.OnDelete != nil {
					s.mode = modeConfirmDelete
				}
			case 5: // Ctrl+E — edit
				if len(s.filtered) > 0 && s.OnUpdate != nil {
					s.mode = modeEditChoose
				}
			case 6: // Ctrl+F — toggle failed only
				s.failOnly = !s.failOnly
				s.applyAndReset()
			case 18: // Ctrl+R — toggle regex mode
				s.useRegex = !s.useRegex
				s.applyAndReset()
			case 127, 8: // Backspace
				if len(s.filter) > 0 {
					runes := []rune(s.filter)
					s.filter = string(runes[:len(runes)-1])
					s.applyAndReset()
				}
			default:
				if buf[0] >= 0x20 && buf[0] <= 0x7E {
					s.filter += string(buf[0])
					s.applyAndReset()
				}
			}
		}
```

- [ ] **Step 7: Run existing tests to verify nothing is broken**

Run: `cd /home/cyb3r/RTLog && go test ./...`
Expected: All PASS. The new callback fields default to nil, so existing callers (show without callbacks) are unaffected.

- [ ] **Step 8: Build to verify compilation**

Run: `cd /home/cyb3r/RTLog && go build -o rtlog .`
Expected: Clean build, no errors

- [ ] **Step 9: Commit**

```bash
git add internal/display/selector.go
git commit -m "feat(display): add delete/edit modes to TUI selector with callbacks"
```

---

### Task 5: Wire callbacks in `show` command

**Files:**
- Modify: `cmd/show.go:231-237`

- [ ] **Step 1: Set OnDelete and OnUpdate callbacks on the Selector before calling Run()**

In `cmd/show.go`, find the TUI branch (around line 231-237):

```go
		} else if display.IsTTY {
			// Interactive TUI: entries in chronological order (oldest first)
			sel := display.NewSelector(entryMaps)
			if err := sel.Run(); err != nil {
```

Replace with:

```go
		} else if display.IsTTY {
			// Interactive TUI: entries in chronological order (oldest first)
			sel := display.NewSelector(entryMaps)
			sel.OnDelete = func(id int64) error {
				return d.Delete(id)
			}
			sel.OnUpdate = func(id int64, fields map[string]string) error {
				return d.Update(id, fields)
			}
			if err := sel.Run(); err != nil {
```

- [ ] **Step 2: Build and verify**

Run: `cd /home/cyb3r/RTLog && go build -o rtlog .`
Expected: Clean build

- [ ] **Step 3: Commit**

```bash
git add cmd/show.go
git commit -m "feat(show): wire delete/edit callbacks to TUI selector"
```

---

### Task 6: Remove standalone `edit` command

**Files:**
- Delete: `cmd/edit.go`

- [ ] **Step 1: Delete the edit command file**

```bash
rm cmd/edit.go
```

Since `editCmd` is registered via `init()` in `cmd/edit.go` which calls `rootCmd.AddCommand(editCmd)`, deleting the file removes the command entirely. No other file references `editCmd`.

- [ ] **Step 2: Verify no remaining references to editCmd**

Run: `cd /home/cyb3r/RTLog && grep -r "editCmd\|editNote\|editTag" cmd/`
Expected: No output (no references)

- [ ] **Step 3: Build to verify clean compilation**

Run: `cd /home/cyb3r/RTLog && go build -o rtlog .`
Expected: Clean build

- [ ] **Step 4: Verify edit command is gone**

Run: `cd /home/cyb3r/RTLog && ./rtlog edit --help`
Expected: Error — unknown command "edit"

- [ ] **Step 5: Run all tests**

Run: `cd /home/cyb3r/RTLog && go test ./...`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git rm cmd/edit.go
git commit -m "refactor(cmd): remove standalone edit command (superseded by TUI edit)"
```

---

### Task 7: Final integration verification

**Files:** None (manual testing only)

- [ ] **Step 1: Run full test suite**

Run: `cd /home/cyb3r/RTLog && go test ./... -v`
Expected: All PASS

- [ ] **Step 2: Smoke test rm command**

```bash
./rtlog new test-rm-smoke
./rtlog rm test-rm-smoke -y
./rtlog list  # should not show test-rm-smoke
```

- [ ] **Step 3: Smoke test rename command**

```bash
./rtlog new test-rename-old
./rtlog rename test-rename-old test-rename-new
./rtlog list   # should show test-rename-new, not test-rename-old
./rtlog status # should show test-rename-new as active
./rtlog rm test-rename-new -y  # cleanup
```

- [ ] **Step 4: Smoke test TUI delete/edit**

```bash
./rtlog new test-tui
# Log a few test entries (or check existing engagement)
./rtlog show
# In the TUI:
#   Ctrl+E → t → type "recon" → Enter   (should update tag)
#   Ctrl+E → n → type "test note" → Enter (should update note)
#   Ctrl+D → y                             (should delete entry)
#   Esc to quit
./rtlog rm test-tui -y  # cleanup
```

- [ ] **Step 5: Verify edit command is removed**

Run: `./rtlog edit 1 --note test`
Expected: `Error: unknown command "edit" for "rtlog"`
