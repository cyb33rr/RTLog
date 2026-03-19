# Absorb Search Into Show — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the `search` command and absorb its functionality into `show` via an optional positional keyword argument.

**Architecture:** The `show` command gains `cobra.MaximumNArgs(1)`. When a keyword is present, entries are fetched via `db.Search` or the new `db.SearchByDate`, formatted with `FmtEntryHighlight`, and printed non-interactively. Existing `show` behavior (TUI, `-a`, pipe) is unchanged when no keyword is given.

**Tech Stack:** Go, Cobra, SQLite (modernc.org/sqlite)

**Spec:** `docs/superpowers/specs/2026-03-19-absorb-search-into-show.md`

---

### Task 1: Narrow `db.Search` from 7 fields to 5

**Files:**
- Modify: `internal/db/db.go:135-146`
- Modify: `internal/db/db_test.go:230-254`

- [ ] **Step 1: Update `TestSearchUser` to expect 0 results**

Replace the test to verify that searching by `user` field no longer matches. The test name changes to `TestSearchDoesNotMatchUserHost` to document the intentional exclusion.

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/cyb3r/RTLog && go test ./internal/db/ -run TestSearchDoesNotMatchUserHost -v`
Expected: FAIL — `Search("alice")` currently returns 1 result because `user` is still in the WHERE clause.

- [ ] **Step 3: Update `db.Search` to query 5 fields**

In `internal/db/db.go`, replace the `Search` method:

```go
// Search returns entries matching the keyword across cmd, tool, cwd, tag, and note fields.
func (d *DB) Search(keyword string) ([]logfile.LogEntry, error) {
	pattern := "%" + keyword + "%"
	return d.queryEntries(
		`SELECT id, ts, epoch, user, host, tty, cwd, tool, cmd, exit, dur, tag, note, out
		 FROM entries
		 WHERE cmd  LIKE ? OR tool LIKE ? OR cwd  LIKE ?
		    OR tag  LIKE ? OR note LIKE ?
		 ORDER BY id ASC`,
		pattern, pattern, pattern, pattern, pattern,
	)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cyb3r/RTLog && go test ./internal/db/ -v`
Expected: All tests pass, including the updated `TestSearchDoesNotMatchUserHost` and the existing `TestSearch`.

- [ ] **Step 5: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go
git commit -m "refactor(db): narrow Search to 5 fields, drop user and host"
```

---

### Task 2: Add `db.SearchByDate`

**Files:**
- Modify: `internal/db/db.go` (add method after `Search`)
- Modify: `internal/db/db_test.go` (add test)

- [ ] **Step 1: Write the failing test**

Append to `internal/db/db_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/cyb3r/RTLog && go test ./internal/db/ -run TestSearchByDate -v`
Expected: FAIL — `SearchByDate` method does not exist yet.

- [ ] **Step 3: Implement `SearchByDate`**

In `internal/db/db.go`, add after the `Search` method:

```go
// SearchByDate returns entries matching the keyword whose ts starts with dateStr.
func (d *DB) SearchByDate(keyword, dateStr string) ([]logfile.LogEntry, error) {
	pattern := "%" + keyword + "%"
	return d.queryEntries(
		`SELECT id, ts, epoch, user, host, tty, cwd, tool, cmd, exit, dur, tag, note, out
		 FROM entries
		 WHERE ts LIKE ?
		   AND (cmd LIKE ? OR tool LIKE ? OR cwd LIKE ? OR tag LIKE ? OR note LIKE ?)
		 ORDER BY id ASC`,
		dateStr+"%", pattern, pattern, pattern, pattern, pattern,
	)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cyb3r/RTLog && go test ./internal/db/ -v`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go
git commit -m "feat(db): add SearchByDate for combined date + keyword queries"
```

---

### Task 3: Add `showOutIndicator` param to `FmtEntryHighlight`

**Files:**
- Modify: `internal/display/format.go:72-84`

- [ ] **Step 1: Update `FmtEntryHighlight` signature**

In `internal/display/format.go`, replace the `FmtEntryHighlight` function:

```go
// FmtEntryHighlight formats an entry then highlights pattern matches.
func FmtEntryHighlight(entry Entry, pattern *regexp.Regexp, index, idxWidth int, showOutIndicator ...bool) string {
	line := FmtEntry(entry, index, idxWidth, showOutIndicator...)
	if pattern == nil {
		return line
	}
	if IsTTY {
		return pattern.ReplaceAllStringFunc(line, func(match string) string {
			return Magenta + Bold + match + Reset
		})
	}
	return line
}
```

- [ ] **Step 2: Run existing tests to verify nothing breaks**

Run: `cd /home/cyb3r/RTLog && go test ./internal/display/ -v`
Expected: All tests pass. The variadic param is backward-compatible — existing callers that pass no value continue to work.

- [ ] **Step 3: Commit**

```bash
git add internal/display/format.go
git commit -m "feat(display): add showOutIndicator param to FmtEntryHighlight"
```

---

### Task 4: Absorb search into `show` command

**Files:**
- Modify: `cmd/show.go`

- [ ] **Step 1: Add `regexp` to imports**

Add `"regexp"` to the import block in `cmd/show.go`:

```go
import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/spf13/cobra"
)
```

- [ ] **Step 2: Update command metadata and add `Args`**

Change the `showCmd` definition:

```go
var showCmd = &cobra.Command{
	Use:   "show [keyword]",
	Short: "Pretty-print log entries",
	Long:  "Display log entries in a human-readable format, optionally filtered by date.\nWith a keyword argument, performs a non-interactive search with highlighting.",
	Args:  cobra.MaximumNArgs(1),
```

- [ ] **Step 3: Add keyword search branch to the Run function**

Inside the `Run` function, after the `entries` slice is populated and the empty check, add the keyword branch. The full `Run` function body becomes:

```go
	Run: func(cmd *cobra.Command, args []string) {
		// Validate date flag early
		if showDate != "" {
			if _, err := time.Parse("2006-01-02", showDate); err != nil {
				fmt.Fprintf(os.Stderr, "Invalid date format: %s (expected YYYY-MM-DD)\n", showDate)
				os.Exit(1)
			}
		}

		path, err := logfile.GetLogPath(engagementFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		d, err := openEngagementDB()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer d.Close()

		engName := logfile.EngagementName(path)
		keyword := ""
		if len(args) > 0 {
			keyword = args[0]
		}

		// --- Keyword search branch ---
		if keyword != "" {
			var dateLabel string
			var matches []logfile.LogEntry
			if showDate != "" {
				matches, err = d.SearchByDate(keyword, showDate)
				dateLabel = showDate
			} else if showToday {
				today := time.Now().UTC().Format("2006-01-02")
				matches, err = d.SearchByDate(keyword, today)
				dateLabel = today
			} else {
				matches, err = d.Search(keyword)
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error searching entries: %v\n", err)
				os.Exit(1)
			}

			if len(matches) == 0 {
				label := ""
				if dateLabel != "" {
					label = fmt.Sprintf(" for %s", dateLabel)
				}
				fmt.Printf("No matches for '%s'%s in %s\n", keyword, label, engName)
				return
			}

			header := fmt.Sprintf("--- %d match(es) for '%s' in %s ---", len(matches), keyword, engName)
			if dateLabel != "" {
				header += fmt.Sprintf("  [%s]", dateLabel)
			}
			fmt.Println(display.Colorize(header, display.Bold))
			fmt.Println()

			pattern := regexp.MustCompile("(?i)" + regexp.QuoteMeta(keyword))
			idxWidth := len(fmt.Sprintf("%d", len(matches)))
			for i, entry := range matches {
				m := logfile.ToMap(entry)
				if showOutput {
					fmt.Println(display.FmtEntryHighlight(m, pattern, i+1, idxWidth, false))
					display.PrintOutputBlock(m, true)
				} else {
					fmt.Println(display.FmtEntryHighlight(m, pattern, i+1, idxWidth))
				}
			}

			fmt.Println()
			fmt.Printf("%s\n", display.Colorize(
				fmt.Sprintf("%d result(s)", len(matches)),
				display.Dim,
			))
			return
		}

		// --- No keyword: existing show behavior ---
		var dateLabel string
		var entries []logfile.LogEntry
		if showDate != "" {
			entries, err = d.LoadByDate(showDate)
			dateLabel = showDate
		} else if showToday {
			today := time.Now().UTC().Format("2006-01-02")
			entries, err = d.LoadByDate(today)
			dateLabel = today
		} else {
			entries, err = d.LoadAll()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
			os.Exit(1)
		}

		if len(entries) == 0 {
			label := ""
			if dateLabel != "" {
				label = fmt.Sprintf(" for %s", dateLabel)
			}
			fmt.Printf("No entries found%s in %s\n", label, engName)
			return
		}

		// Convert to display.Entry maps
		entryMaps := make([]display.Entry, len(entries))
		for i, e := range entries {
			entryMaps[i] = logfile.ToMap(e)
		}

		if showOutput {
			// Non-interactive --all: reverse for newest-first, use FmtEntry
			for i, j := 0, len(entryMaps)-1; i < j; i, j = i+1, j-1 {
				entryMaps[i], entryMaps[j] = entryMaps[j], entryMaps[i]
			}
			header := fmt.Sprintf("--- %s ---", engName)
			if dateLabel != "" {
				header += fmt.Sprintf("  [%s]", dateLabel)
			}
			idxWidth := len(fmt.Sprintf("%d", len(entries)))
			n := len(entryMaps)
			origIdx := func(i int) int { return n - i }
			fmt.Println(display.Colorize(header, display.Bold))
			fmt.Println()
			for i, m := range entryMaps {
				fmt.Println(display.FmtEntry(m, origIdx(i), idxWidth, false))
				display.PrintOutputBlock(m, true)
			}
		} else if display.IsTTY {
			// Interactive TUI: entries in chronological order (oldest first)
			sel := display.NewSelector(entryMaps)
			if err := sel.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Non-interactive pipe: reverse for newest-first, use FmtEntry
			for i, j := 0, len(entryMaps)-1; i < j; i, j = i+1, j-1 {
				entryMaps[i], entryMaps[j] = entryMaps[j], entryMaps[i]
			}
			header := fmt.Sprintf("--- %s ---", engName)
			if dateLabel != "" {
				header += fmt.Sprintf("  [%s]", dateLabel)
			}
			idxWidth := len(fmt.Sprintf("%d", len(entries)))
			n := len(entryMaps)
			origIdx := func(i int) int { return n - i }
			fmt.Println(display.Colorize(header, display.Bold))
			fmt.Println()
			for i, m := range entryMaps {
				fmt.Println(display.FmtEntry(m, origIdx(i), idxWidth))
			}
		}
	},
```

- [ ] **Step 4: Verify the project compiles**

Run: `cd /home/cyb3r/RTLog && go build ./...`
Expected: No errors.

- [ ] **Step 5: Commit**

```bash
git add cmd/show.go
git commit -m "feat(show): accept optional keyword for non-interactive search"
```

---

### Task 5: Delete `cmd/search.go`

**Files:**
- Delete: `cmd/search.go`

- [ ] **Step 1: Delete the file**

```bash
rm cmd/search.go
```

- [ ] **Step 2: Verify the project compiles**

Run: `cd /home/cyb3r/RTLog && go build ./...`
Expected: No errors. `search.go` registered itself via `init()` calling `rootCmd.AddCommand(searchCmd)`, so removing the file removes the command cleanly.

- [ ] **Step 3: Commit**

```bash
git add cmd/search.go
git commit -m "refactor: remove search command (absorbed into show)"
```

---

### Task 6: Update README

**Files:**
- Modify: `README.md:98-105`

- [ ] **Step 1: Update the "Viewing Logs" section**

Replace the current "Viewing Logs" block in `README.md`:

```markdown
### Viewing Logs

```bash
rtlog show               # Interactive selector (navigate with ↑/↓, Enter to view output)
rtlog show --today       # Today's entries only
rtlog show --date 2026-01-15
rtlog show -a            # Print all entries with output (non-interactive)
rtlog show nmap          # Search entries matching keyword (non-interactive)
rtlog show --today nmap  # Search within today's entries
rtlog show -e <name>     # Show a different engagement
```
```

- [ ] **Step 2: Update the "Search & analysis" feature line**

In the Features section near line 12, change:

```markdown
- **Search & analysis** — search logs, view timelines, get tool usage stats
```

to:

```markdown
- **Search & analysis** — filter and search logs, view timelines, get tool usage stats
```

- [ ] **Step 3: Verify no other references to `rtlog search` remain**

Run: `grep -rn "rtlog search" README.md`
Expected: No output.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: update README for search absorption into show"
```

---

### Task 7: Final verification

- [ ] **Step 1: Run all tests**

Run: `cd /home/cyb3r/RTLog && go test ./... -v`
Expected: All tests pass.

- [ ] **Step 2: Build the binary**

Run: `cd /home/cyb3r/RTLog && go build -o rtlog .`
Expected: Compiles successfully.

- [ ] **Step 3: Verify `rtlog show --help` shows keyword arg**

Run: `cd /home/cyb3r/RTLog && ./rtlog show --help`
Expected: Usage line shows `rtlog show [keyword]` and Long description mentions keyword search.

- [ ] **Step 4: Verify `rtlog search` no longer exists**

Run: `cd /home/cyb3r/RTLog && ./rtlog search nmap 2>&1`
Expected: Error like `unknown command "search"`.
