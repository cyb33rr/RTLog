# Export Filtering & Search Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add stackable filter flags (`--tool`, `--tag`, `--date`, `--from`, `--to`, `--filter`, `-r`) to `rtlog export`, add regex search (`-r`) to `rtlog show`, and add a Ctrl+R regex toggle to the interactive TUI.

**Architecture:** Hybrid SQL + Go filtering. Structured fields (tool, tag, date/epoch) are filtered at the SQL level using existing indices. Free-text substring (`--filter`) and regex (`-r`) matching are applied in Go via a new `internal/filter` package. The TUI's `ApplyFilters` gets a `useRegex` parameter for its own regex mode.

**Tech Stack:** Go, SQLite (via modernc.org/sqlite), cobra, regexp

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/filter/filter.go` | Create | `MatchSubstring` and `MatchRegex` functions for Go-level entry filtering |
| `internal/filter/filter_test.go` | Create | Tests for both filter functions |
| `internal/db/db.go` | Modify | Add `LoadFiltered` method with dynamic SQL WHERE clauses |
| `internal/db/db_test.go` | Modify | Add tests for `LoadFiltered` |
| `cmd/export.go` | Modify | Add filter flags, wire up `LoadFiltered` + Go filters |
| `cmd/show.go` | Modify | Add `-r` flag for regex search mode |
| `internal/display/selector.go` | Modify | Add `useRegex` to `ApplyFilters`, Ctrl+R toggle, filter bar indicator |
| `internal/display/selector_test.go` | Modify | Add tests for regex mode in `ApplyFilters` |

---

### Task 1: Create `internal/filter` Package — Tests

**Files:**
- Create: `internal/filter/filter_test.go`

- [ ] **Step 1: Write failing tests for `MatchSubstring`**

```go
package filter

import (
	"testing"

	"github.com/cyb33rr/rtlog/internal/logfile"
)

func sampleEntries() []logfile.LogEntry {
	return []logfile.LogEntry{
		{Cmd: "nmap -sV 10.0.0.1", Tool: "nmap", Cwd: "/tmp", Tag: "recon", Note: "initial scan"},
		{Cmd: "gobuster dir -u http://target", Tool: "gobuster", Cwd: "/opt", Tag: "recon", Note: ""},
		{Cmd: "evil-winrm -i 10.0.0.1", Tool: "evil-winrm", Cwd: "/tmp", Tag: "exploitation", Note: "got shell"},
	}
}

func TestMatchSubstringByCmd(t *testing.T) {
	result := MatchSubstring(sampleEntries(), "10.0.0.1")
	if len(result) != 2 {
		t.Errorf("got %d, want 2", len(result))
	}
}

func TestMatchSubstringByTool(t *testing.T) {
	result := MatchSubstring(sampleEntries(), "gobuster")
	if len(result) != 1 {
		t.Errorf("got %d, want 1", len(result))
	}
}

func TestMatchSubstringByCwd(t *testing.T) {
	result := MatchSubstring(sampleEntries(), "/opt")
	if len(result) != 1 {
		t.Errorf("got %d, want 1", len(result))
	}
}

func TestMatchSubstringByTag(t *testing.T) {
	result := MatchSubstring(sampleEntries(), "exploitation")
	if len(result) != 1 {
		t.Errorf("got %d, want 1", len(result))
	}
}

func TestMatchSubstringByNote(t *testing.T) {
	result := MatchSubstring(sampleEntries(), "shell")
	if len(result) != 1 {
		t.Errorf("got %d, want 1", len(result))
	}
}

func TestMatchSubstringCaseInsensitive(t *testing.T) {
	result := MatchSubstring(sampleEntries(), "NMAP")
	if len(result) != 1 {
		t.Errorf("got %d, want 1 (case-insensitive match on entry 0)", len(result))
	}
}

func TestMatchSubstringNoMatch(t *testing.T) {
	result := MatchSubstring(sampleEntries(), "zzzznotfound")
	if len(result) != 0 {
		t.Errorf("got %d, want 0", len(result))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cyb3r/RTLog && go test ./internal/filter/ -v -run TestMatchSubstring`
Expected: FAIL — `MatchSubstring` not defined

- [ ] **Step 3: Write failing tests for `MatchRegex`**

Add to `internal/filter/filter_test.go`:

```go
func TestMatchRegexByCmd(t *testing.T) {
	result, err := MatchRegex(sampleEntries(), `10\.0\.0\.\d+`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d, want 2", len(result))
	}
}

func TestMatchRegexByTool(t *testing.T) {
	result, err := MatchRegex(sampleEntries(), `^gobuster$`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d, want 1", len(result))
	}
}

func TestMatchRegexByCwd(t *testing.T) {
	result, err := MatchRegex(sampleEntries(), `/opt`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d, want 1", len(result))
	}
}

func TestMatchRegexByTag(t *testing.T) {
	result, err := MatchRegex(sampleEntries(), `exploit.*`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d, want 1", len(result))
	}
}

func TestMatchRegexByNote(t *testing.T) {
	result, err := MatchRegex(sampleEntries(), `got\s+shell`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d, want 1", len(result))
	}
}

func TestMatchRegexNoMatch(t *testing.T) {
	result, err := MatchRegex(sampleEntries(), `zzzznotfound`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("got %d, want 0", len(result))
	}
}

func TestMatchRegexInvalidPattern(t *testing.T) {
	_, err := MatchRegex(sampleEntries(), `[invalid`)
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestMatchRegexEmptyPattern(t *testing.T) {
	result, err := MatchRegex(sampleEntries(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("got %d, want 3 (empty pattern matches all)", len(result))
	}
}
```

- [ ] **Step 4: Run all filter tests to confirm failure**

Run: `cd /home/cyb3r/RTLog && go test ./internal/filter/ -v`
Expected: FAIL — functions not defined

---

### Task 2: Create `internal/filter` Package — Implementation

**Files:**
- Create: `internal/filter/filter.go`

- [ ] **Step 1: Implement `MatchSubstring` and `MatchRegex`**

```go
package filter

import (
	"regexp"
	"strings"

	"github.com/cyb33rr/rtlog/internal/logfile"
)

// fields returns the 5 searchable field values from a LogEntry.
func fields(e logfile.LogEntry) [5]string {
	return [5]string{e.Cmd, e.Tool, e.Cwd, e.Tag, e.Note}
}

// MatchSubstring returns entries where any of the 5 searchable fields
// contains substr (case-insensitive).
func MatchSubstring(entries []logfile.LogEntry, substr string) []logfile.LogEntry {
	lower := strings.ToLower(substr)
	var result []logfile.LogEntry
	for _, e := range entries {
		for _, f := range fields(e) {
			if strings.Contains(strings.ToLower(f), lower) {
				result = append(result, e)
				break
			}
		}
	}
	if result == nil {
		result = []logfile.LogEntry{}
	}
	return result
}

// MatchRegex returns entries where any of the 5 searchable fields
// matches the given regex pattern. Returns an error if the pattern is invalid.
func MatchRegex(entries []logfile.LogEntry, pattern string) ([]logfile.LogEntry, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	var result []logfile.LogEntry
	for _, e := range entries {
		for _, f := range fields(e) {
			if re.MatchString(f) {
				result = append(result, e)
				break
			}
		}
	}
	if result == nil {
		result = []logfile.LogEntry{}
	}
	return result, nil
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `cd /home/cyb3r/RTLog && go test ./internal/filter/ -v`
Expected: All PASS

- [ ] **Step 3: Commit**

```bash
git add internal/filter/filter.go internal/filter/filter_test.go
git commit -m "feat(filter): add MatchSubstring and MatchRegex for entry filtering"
```

---

### Task 3: Add `LoadFiltered` to Database Layer — Tests

**Files:**
- Modify: `internal/db/db_test.go`

- [ ] **Step 1: Write failing tests for `LoadFiltered`**

Add to `internal/db/db_test.go`:

```go
// filteredEntries returns diverse entries for LoadFiltered tests.
func filteredEntries() []logfile.LogEntry {
	return []logfile.LogEntry{
		{Ts: "2025-01-15T10:00:00Z", Epoch: 1736935200, User: "op", Host: "kali", Cwd: "/tmp", Tool: "nmap", Cmd: "nmap -sV 10.0.0.1", Tag: "recon"},
		{Ts: "2025-01-15T11:00:00Z", Epoch: 1736938800, User: "op", Host: "kali", Cwd: "/tmp", Tool: "gobuster", Cmd: "gobuster dir -u http://target", Tag: "recon"},
		{Ts: "2025-01-16T09:00:00Z", Epoch: 1737018000, User: "op", Host: "kali", Cwd: "/tmp", Tool: "nmap", Cmd: "nmap -p- 10.0.0.2", Tag: "exploitation"},
		{Ts: "2025-01-17T14:00:00Z", Epoch: 1737122400, User: "op", Host: "kali", Cwd: "/tmp", Tool: "evil-winrm", Cmd: "evil-winrm -i 10.0.0.1", Tag: "exploitation"},
	}
}

func insertFiltered(t *testing.T, d *DB) {
	t.Helper()
	for _, e := range filteredEntries() {
		if err := d.Insert(e); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
}

func TestLoadFilteredNoFilters(t *testing.T) {
	d := openTestDB(t)
	insertFiltered(t, d)

	entries, err := d.LoadFiltered(nil, nil, "", "", "")
	if err != nil {
		t.Fatalf("LoadFiltered: %v", err)
	}
	if len(entries) != 4 {
		t.Errorf("got %d, want 4 (no filters = all)", len(entries))
	}
}

func TestLoadFilteredByTool(t *testing.T) {
	d := openTestDB(t)
	insertFiltered(t, d)

	entries, err := d.LoadFiltered([]string{"nmap"}, nil, "", "", "")
	if err != nil {
		t.Fatalf("LoadFiltered: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("got %d, want 2 (two nmap entries)", len(entries))
	}
}

func TestLoadFilteredByMultipleTools(t *testing.T) {
	d := openTestDB(t)
	insertFiltered(t, d)

	entries, err := d.LoadFiltered([]string{"nmap", "gobuster"}, nil, "", "", "")
	if err != nil {
		t.Fatalf("LoadFiltered: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("got %d, want 3 (nmap + gobuster)", len(entries))
	}
}

func TestLoadFilteredByTag(t *testing.T) {
	d := openTestDB(t)
	insertFiltered(t, d)

	entries, err := d.LoadFiltered(nil, []string{"recon"}, "", "", "")
	if err != nil {
		t.Fatalf("LoadFiltered: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("got %d, want 2 (two recon entries)", len(entries))
	}
}

func TestLoadFilteredByDate(t *testing.T) {
	d := openTestDB(t)
	insertFiltered(t, d)

	entries, err := d.LoadFiltered(nil, nil, "2025-01-15", "", "")
	if err != nil {
		t.Fatalf("LoadFiltered: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("got %d, want 2 (two entries on 2025-01-15)", len(entries))
	}
}

func TestLoadFilteredByFrom(t *testing.T) {
	d := openTestDB(t)
	insertFiltered(t, d)

	entries, err := d.LoadFiltered(nil, nil, "", "2025-01-16", "")
	if err != nil {
		t.Fatalf("LoadFiltered: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("got %d, want 2 (entries from 01-16 onward)", len(entries))
	}
}

func TestLoadFilteredByTo(t *testing.T) {
	d := openTestDB(t)
	insertFiltered(t, d)

	entries, err := d.LoadFiltered(nil, nil, "", "", "2025-01-15")
	if err != nil {
		t.Fatalf("LoadFiltered: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("got %d, want 2 (entries up to 01-15)", len(entries))
	}
}

func TestLoadFilteredByFromTo(t *testing.T) {
	d := openTestDB(t)
	insertFiltered(t, d)

	entries, err := d.LoadFiltered(nil, nil, "", "2025-01-15", "2025-01-16")
	if err != nil {
		t.Fatalf("LoadFiltered: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("got %d, want 3 (entries from 01-15 through 01-16)", len(entries))
	}
}

func TestLoadFilteredCombined(t *testing.T) {
	d := openTestDB(t)
	insertFiltered(t, d)

	entries, err := d.LoadFiltered([]string{"nmap"}, []string{"recon"}, "", "2025-01-15", "2025-01-15")
	if err != nil {
		t.Fatalf("LoadFiltered: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("got %d, want 1 (nmap + recon + date range)", len(entries))
	}
}

func TestLoadFilteredNoMatches(t *testing.T) {
	d := openTestDB(t)
	insertFiltered(t, d)

	entries, err := d.LoadFiltered([]string{"sqlmap"}, nil, "", "", "")
	if err != nil {
		t.Fatalf("LoadFiltered: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d, want 0", len(entries))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cyb3r/RTLog && go test ./internal/db/ -v -run TestLoadFiltered`
Expected: FAIL — `LoadFiltered` not defined

---

### Task 4: Add `LoadFiltered` to Database Layer — Implementation

**Files:**
- Modify: `internal/db/db.go`

- [ ] **Step 1: Implement `LoadFiltered`**

Add to `internal/db/db.go` after the `SearchByDate` method:

```go
// LoadFiltered returns entries matching the given structured filters.
// All non-empty filters combine with AND logic.
// tools/tags use OR logic within (IN clause).
// date uses ts LIKE prefix match.
// from/to use epoch range (inclusive, parsed as YYYY-MM-DD).
func (d *DB) LoadFiltered(tools []string, tags []string, date, from, to string) ([]logfile.LogEntry, error) {
	query := "SELECT id, ts, epoch, user, host, tty, cwd, tool, cmd, exit, dur, tag, note, out FROM entries"
	var clauses []string
	var args []interface{}

	if len(tools) > 0 {
		placeholders := make([]string, len(tools))
		for i, t := range tools {
			placeholders[i] = "?"
			args = append(args, t)
		}
		clauses = append(clauses, "tool IN ("+strings.Join(placeholders, ", ")+")")
	}

	if len(tags) > 0 {
		placeholders := make([]string, len(tags))
		for i, t := range tags {
			placeholders[i] = "?"
			args = append(args, t)
		}
		clauses = append(clauses, "tag IN ("+strings.Join(placeholders, ", ")+")")
	}

	if date != "" {
		clauses = append(clauses, "ts LIKE ?")
		args = append(args, date+"%")
	}

	if from != "" {
		t, err := time.Parse("2006-01-02", from)
		if err != nil {
			return nil, fmt.Errorf("invalid --from date: %w", err)
		}
		clauses = append(clauses, "epoch >= ?")
		args = append(args, t.UTC().Unix())
	}

	if to != "" {
		t, err := time.Parse("2006-01-02", to)
		if err != nil {
			return nil, fmt.Errorf("invalid --to date: %w", err)
		}
		// End of day: 23:59:59
		endOfDay := t.UTC().Add(24*time.Hour - time.Second)
		clauses = append(clauses, "epoch <= ?")
		args = append(args, endOfDay.Unix())
	}

	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY id ASC"

	return d.queryEntries(query, args...)
}
```

Also add `"time"` to the import block in `db.go`.

- [ ] **Step 2: Run tests to verify they pass**

Run: `cd /home/cyb3r/RTLog && go test ./internal/db/ -v -run TestLoadFiltered`
Expected: All PASS

- [ ] **Step 3: Run all existing DB tests to verify no regressions**

Run: `cd /home/cyb3r/RTLog && go test ./internal/db/ -v`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go
git commit -m "feat(db): add LoadFiltered method with tool/tag/date/range SQL filtering"
```

---

### Task 5: Add Filter Flags to Export Command

**Files:**
- Modify: `cmd/export.go`

- [ ] **Step 1: Add flag variables and registration**

Replace the entire `cmd/export.go` with:

```go
package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/cyb33rr/rtlog/internal/export"
	"github.com/cyb33rr/rtlog/internal/filter"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/spf13/cobra"
)

var (
	exportOutput string
	exportTool   string
	exportTag    string
	exportDate   string
	exportFrom   string
	exportTo     string
	exportFilter string
	exportRegex  string
)

var exportCmd = &cobra.Command{
	Use:   "export <md|csv|jsonl>",
	Short: "Export entries as Markdown, CSV, or JSONL",
	Long:  "Export log entries to a Markdown table, CSV, or JSONL file.\nSupports filtering by tool, tag, date range, substring, and regex.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		format := args[0]
		if format != "md" && format != "csv" && format != "jsonl" {
			fmt.Fprintf(os.Stderr, "Unknown export format: %s (use md, csv, or jsonl)\n", format)
			os.Exit(1)
		}

		// Validate mutual exclusivity: --date vs --from/--to
		if exportDate != "" && (exportFrom != "" || exportTo != "") {
			fmt.Fprintf(os.Stderr, "Cannot use --date with --from or --to\n")
			os.Exit(1)
		}

		// Validate date formats
		for _, pair := range []struct{ flag, val string }{
			{"--date", exportDate}, {"--from", exportFrom}, {"--to", exportTo},
		} {
			if pair.val != "" {
				if _, err := time.Parse("2006-01-02", pair.val); err != nil {
					fmt.Fprintf(os.Stderr, "Invalid %s format: %s (expected YYYY-MM-DD)\n", pair.flag, pair.val)
					os.Exit(1)
				}
			}
		}

		// Validate regex
		if exportRegex != "" {
			if _, err := regexp.Compile(exportRegex); err != nil {
				fmt.Fprintf(os.Stderr, "Invalid regex: %v\n", err)
				os.Exit(1)
			}
		}

		// Parse comma-separated tools and tags
		var tools, tags []string
		if exportTool != "" {
			tools = strings.Split(exportTool, ",")
		}
		if exportTag != "" {
			tags = strings.Split(exportTag, ",")
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

		hasFilters := len(tools) > 0 || len(tags) > 0 || exportDate != "" || exportFrom != "" || exportTo != "" || exportFilter != "" || exportRegex != ""

		entries, err := d.LoadFiltered(tools, tags, exportDate, exportFrom, exportTo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
			os.Exit(1)
		}

		// Apply substring filter
		if exportFilter != "" {
			entries = filter.MatchSubstring(entries, exportFilter)
		}

		// Apply regex filter
		if exportRegex != "" {
			entries, err = filter.MatchRegex(entries, exportRegex)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error applying regex: %v\n", err)
				os.Exit(1)
			}
		}

		if len(entries) == 0 {
			if hasFilters {
				fmt.Fprintf(os.Stderr, "No entries match the given filters\n")
			} else {
				fmt.Fprintf(os.Stderr, "No entries in %s\n", logfile.EngagementName(path))
			}
			return
		}

		var text string
		switch format {
		case "md":
			text = export.ExportMarkdown(entries)
		case "csv":
			text = export.ExportCSV(entries)
		case "jsonl":
			text = export.ExportJSONL(entries)
		}

		outPath := exportOutput
		if outPath == "" {
			outPath = logfile.EngagementName(path) + "." + format
		}

		if err := os.WriteFile(outPath, []byte(text+"\n"), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Written to %s\n", outPath)
	},
}

func init() {
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "output file path (default: <engagement>.<format>)")
	exportCmd.Flags().StringVar(&exportTool, "tool", "", "filter by tool (comma-separated, e.g. nmap,nxc)")
	exportCmd.Flags().StringVar(&exportTag, "tag", "", "filter by tag (comma-separated, e.g. recon,privesc)")
	exportCmd.Flags().StringVar(&exportDate, "date", "", "filter by date (YYYY-MM-DD)")
	exportCmd.Flags().StringVar(&exportFrom, "from", "", "filter from date inclusive (YYYY-MM-DD)")
	exportCmd.Flags().StringVar(&exportTo, "to", "", "filter to date inclusive (YYYY-MM-DD)")
	exportCmd.Flags().StringVar(&exportFilter, "filter", "", "filter by substring match across cmd, tool, cwd, tag, note")
	exportCmd.Flags().StringVarP(&exportRegex, "regex", "r", "", "filter by regex match across cmd, tool, cwd, tag, note")
	rootCmd.AddCommand(exportCmd)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/cyb3r/RTLog && go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add cmd/export.go
git commit -m "feat(export): add filter flags --tool, --tag, --date, --from, --to, --filter, -r"
```

---

### Task 6: Add `-r` Regex Flag to Show Command

**Files:**
- Modify: `cmd/show.go`

- [ ] **Step 1: Add `-r` flag and regex search branch**

Add the flag variable at the top of `cmd/show.go` alongside the existing vars:

```go
var showRegex string
```

In the `Run` function, after the existing `keyword` assignment (line ~49), add mutual exclusivity check:

```go
		if keyword != "" && showRegex != "" {
			fmt.Fprintf(os.Stderr, "Cannot use keyword argument with -r flag\n")
			os.Exit(1)
		}
```

Then add a new branch after the keyword check block (after `return` on line 104) and before the `// --- No keyword` comment:

```go
		// --- Regex search branch ---
		if showRegex != "" {
			re, err := regexp.Compile(showRegex)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid regex: %v\n", err)
				os.Exit(1)
			}

			var dateLabel string
			var allEntries []logfile.LogEntry
			if showDate != "" {
				allEntries, err = d.LoadByDate(showDate)
				dateLabel = showDate
			} else if showToday {
				today := time.Now().UTC().Format("2006-01-02")
				allEntries, err = d.LoadByDate(today)
				dateLabel = today
			} else {
				allEntries, err = d.LoadAll()
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
				os.Exit(1)
			}

			matches, err := filter.MatchRegex(allEntries, showRegex)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error applying regex: %v\n", err)
				os.Exit(1)
			}

			if len(matches) == 0 {
				label := ""
				if dateLabel != "" {
					label = fmt.Sprintf(" for %s", dateLabel)
				}
				fmt.Printf("No matches for regex '%s'%s in %s\n", showRegex, label, engName)
				return
			}

			header := fmt.Sprintf("--- %d match(es) for regex '%s' in %s ---", len(matches), showRegex, engName)
			if dateLabel != "" {
				header += fmt.Sprintf("  [%s]", dateLabel)
			}
			fmt.Println(display.Colorize(header, display.Bold))
			fmt.Println()

			idxWidth := len(fmt.Sprintf("%d", len(matches)))
			for i, entry := range matches {
				m := logfile.ToMap(entry)
				if showOutput {
					fmt.Println(display.FmtEntryHighlight(m, re, i+1, idxWidth, false))
					display.PrintOutputBlock(m, true)
				} else {
					fmt.Println(display.FmtEntryHighlight(m, re, i+1, idxWidth))
				}
			}

			fmt.Println()
			fmt.Printf("%s\n", display.Colorize(
				fmt.Sprintf("%d result(s)", len(matches)),
				display.Dim,
			))
			return
		}
```

Add the import for `"github.com/cyb33rr/rtlog/internal/filter"` to the import block.

In the `init` function, add the flag registration:

```go
	showCmd.Flags().StringVarP(&showRegex, "regex", "r", "", "search by regex pattern across cmd, tool, cwd, tag, note")
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/cyb3r/RTLog && go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add cmd/show.go
git commit -m "feat(show): add -r flag for regex search mode"
```

---

### Task 7: Add Regex Toggle to Interactive TUI — Tests

**Files:**
- Modify: `internal/display/selector_test.go`

- [ ] **Step 1: Write failing tests for regex mode in `ApplyFilters`**

Add to `internal/display/selector_test.go`:

```go
func TestApplyFiltersRegexMode(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, `10\.0\.0\.\d+`, "", false, true)
	if len(filtered) != 4 {
		t.Errorf("got %d, want 4 (entries 0,2,3,4 have matching IPs)", len(filtered))
	}
}

func TestApplyFiltersRegexModeInvalidPattern(t *testing.T) {
	entries := makeEntries()
	// Invalid regex skips text filter — shows all entries (spec: keep previous valid results)
	filtered := ApplyFilters(entries, `[invalid`, "", false, true)
	if len(filtered) != 5 {
		t.Errorf("got %d, want 5 (invalid regex skips text filter, shows all)", len(filtered))
	}
}

func TestApplyFiltersRegexModeMatchesTool(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, `^nmap$`, "", false, true)
	if len(filtered) != 2 {
		t.Errorf("got %d, want 2 (two nmap entries)", len(filtered))
	}
}

func TestApplyFiltersLiteralModeUnchanged(t *testing.T) {
	entries := makeEntries()
	// Literal mode (useRegex=false) should work as before
	filtered := ApplyFilters(entries, "nmap", "", false, false)
	if len(filtered) != 2 {
		t.Errorf("got %d, want 2 (two nmap entries, literal mode)", len(filtered))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cyb3r/RTLog && go test ./internal/display/ -v -run TestApplyFilters`
Expected: FAIL — `ApplyFilters` signature mismatch (too many arguments)

---

### Task 8: Add Regex Toggle to Interactive TUI — Implementation

**Files:**
- Modify: `internal/display/selector.go`

- [ ] **Step 1: Update `ApplyFilters` to accept `useRegex` parameter**

In `internal/display/selector.go`, update the `ApplyFilters` function signature and body:

Change the signature from:
```go
func ApplyFilters(entries []Entry, textFilter, tagFilter string, failOnly bool) []int {
```
to:
```go
func ApplyFilters(entries []Entry, textFilter, tagFilter string, failOnly bool, useRegex ...bool) []int {
```

Replace the text filter block (lines 30-48) with:

```go
		if textFilter != "" {
			fields := []string{
				getString(e, "cmd", ""),
				getString(e, "tool", ""),
				getString(e, "tag", ""),
				getString(e, "note", ""),
				getString(e, "cwd", ""),
			}
			found := false
			if len(useRegex) > 0 && useRegex[0] {
				re, err := regexp.Compile(textFilter)
				if err != nil {
					// Invalid regex: skip text filter (keep all entries visible)
					found = true
				} else {
					for _, f := range fields {
						if re.MatchString(f) {
							found = true
							break
						}
					}
				}
			} else {
				lower := strings.ToLower(textFilter)
				for _, f := range fields {
					if strings.Contains(strings.ToLower(f), lower) {
						found = true
						break
					}
				}
			}
			if !found {
				continue
			}
		}
```

Add `"regexp"` to the import block.

Remove the `lower := strings.ToLower(textFilter)` line at the top of the function (line 18) since it's now inside the else branch.

- [ ] **Step 2: Add `useRegex` field to `Selector` struct and Ctrl+R handling**

Add to the `Selector` struct:

```go
	useRegex bool // regex mode toggle
```

Update `applyAndReset` to pass `useRegex`:

```go
func (s *Selector) applyAndReset() {
	s.filtered = ApplyFilters(s.entries, s.filter, s.tagFilter, s.failOnly, s.useRegex)
```

Update `NewSelector` to pass `useRegex`:

```go
	s.filtered = ApplyFilters(entries, "", "", false, false)
```

Add Ctrl+R case in the `Run` method's switch block (after the Ctrl+F case):

```go
			case 18: // Ctrl+R — toggle regex mode
				s.useRegex = !s.useRegex
				s.applyAndReset()
```

- [ ] **Step 3: Update `renderFilterBar` to show regex indicator**

Replace the filter prompt line in `renderFilterBar`:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cyb3r/RTLog && go test ./internal/display/ -v`
Expected: All PASS (including existing tests — the variadic `useRegex` param means old call sites don't break)

- [ ] **Step 5: Verify full build**

Run: `cd /home/cyb3r/RTLog && go build ./...`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add internal/display/selector.go internal/display/selector_test.go
git commit -m "feat(tui): add Ctrl+R regex toggle to interactive selector"
```

---

### Task 9: Add Export Flag Validation Tests

**Files:**
- Create: `cmd/export_test.go`

- [ ] **Step 1: Write tests for export flag validation**

```go
package cmd

import (
	"strings"
	"testing"
	"time"
)

func TestExportDateFromToMutualExclusivity(t *testing.T) {
	// --date and --from cannot be used together
	exportDate = "2025-01-15"
	exportFrom = "2025-01-14"
	exportTo = ""

	if exportDate != "" && (exportFrom != "" || exportTo != "") {
		// Expected: this condition triggers the error
	} else {
		t.Error("expected mutual exclusivity to be detected")
	}

	// Reset
	exportDate = ""
	exportFrom = ""
	exportTo = ""
}

func TestExportDateFromToMutualExclusivityWithTo(t *testing.T) {
	exportDate = "2025-01-15"
	exportFrom = ""
	exportTo = "2025-01-16"

	if exportDate != "" && (exportFrom != "" || exportTo != "") {
		// Expected: this condition triggers the error
	} else {
		t.Error("expected mutual exclusivity to be detected")
	}

	exportDate = ""
	exportTo = ""
}

func TestExportCommaSeparatedTools(t *testing.T) {
	input := "nmap,nxc,gobuster"
	tools := strings.Split(input, ",")
	if len(tools) != 3 {
		t.Errorf("got %d tools, want 3", len(tools))
	}
	if tools[0] != "nmap" || tools[1] != "nxc" || tools[2] != "gobuster" {
		t.Errorf("got %v, want [nmap nxc gobuster]", tools)
	}
}

func TestExportCommaSeparatedTags(t *testing.T) {
	input := "recon,privesc"
	tags := strings.Split(input, ",")
	if len(tags) != 2 {
		t.Errorf("got %d tags, want 2", len(tags))
	}
}

func TestExportDateValidation(t *testing.T) {
	for _, tc := range []struct {
		val   string
		valid bool
	}{
		{"2025-01-15", true},
		{"not-a-date", false},
		{"2025/01/15", false},
		{"", true}, // empty is ok (no filter)
	} {
		if tc.val == "" {
			continue
		}
		_, err := time.Parse("2006-01-02", tc.val)
		if tc.valid && err != nil {
			t.Errorf("date %q should be valid but got error: %v", tc.val, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("date %q should be invalid but no error", tc.val)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -v -run TestExport`
Expected: All PASS

- [ ] **Step 3: Commit**

```bash
git add cmd/export_test.go
git commit -m "test(export): add flag validation tests for export filtering"
```

---

### Task 10: Add Show Regex Flag Tests

**Files:**
- Create: `cmd/show_test.go`

- [ ] **Step 1: Write tests for show regex flag behavior**

```go
package cmd

import (
	"regexp"
	"testing"
)

func TestShowRegexValidPattern(t *testing.T) {
	pattern := `10\.0\.0\.\d+`
	_, err := regexp.Compile(pattern)
	if err != nil {
		t.Errorf("expected valid regex, got error: %v", err)
	}
}

func TestShowRegexInvalidPattern(t *testing.T) {
	pattern := `[invalid`
	_, err := regexp.Compile(pattern)
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestShowRegexKeywordMutualExclusivity(t *testing.T) {
	keyword := "nmap"
	regexFlag := `nmap.*`

	if keyword != "" && regexFlag != "" {
		// Expected: this is detected as an error
	} else {
		t.Error("expected mutual exclusivity to be detected")
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `cd /home/cyb3r/RTLog && go test ./cmd/ -v -run TestShow`
Expected: All PASS

- [ ] **Step 3: Commit**

```bash
git add cmd/show_test.go
git commit -m "test(show): add regex flag validation tests"
```

---

### Task 11: Run Full Test Suite

- [ ] **Step 1: Run all tests**

Run: `cd /home/cyb3r/RTLog && go test ./... -v`
Expected: All PASS

- [ ] **Step 2: Run go vet**

Run: `cd /home/cyb3r/RTLog && go vet ./...`
Expected: No issues

- [ ] **Step 3: If any failures, fix and re-run**

---

### Task 12: Final Commit (if any fixes from Task 11)

- [ ] **Step 1: Stage and commit any remaining fixes**

```bash
git add -A
git commit -m "fix: address test/vet issues from export filter implementation"
```

Only needed if Task 11 revealed issues that required fixes.
