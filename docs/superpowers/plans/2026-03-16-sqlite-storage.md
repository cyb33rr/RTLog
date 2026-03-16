# SQLite Storage Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace JSONL file storage with per-engagement SQLite databases using `modernc.org/sqlite`.

**Architecture:** New `internal/db/` package encapsulates all SQLite operations. Shell hooks switch from direct JSONL append to `rtlog log` calls. All read commands use SQL queries instead of loading full JSONL into memory. Import command migrates existing JSONL data.

**Tech Stack:** Go 1.24, `modernc.org/sqlite` (pure Go, CGO-free), `database/sql`

**Spec:** `docs/superpowers/specs/2026-03-16-sqlite-storage-design.md`

**Spec deviation:** `LoadRange(from, to int64)` from the spec is intentionally not implemented. Date filtering uses `LoadByDate(dateStr)` with `ts LIKE 'YYYY-MM-DD%'` instead of epoch ranges, matching the spec's own date filtering section and avoiding timezone ambiguity. `LoadRange` can be added later if epoch-based queries are needed.

---

## File Structure

### New Files
- `internal/db/db.go` — SQLite open, schema, insert, query, close
- `internal/db/db_test.go` — Unit tests for all db operations
- `cmd/importcmd.go` — `rtlog import` command for JSONL migration

### Modified Files
- `go.mod` / `go.sum` — Add `modernc.org/sqlite` dependency
- `internal/logfile/logfile.go` — Add ID to LogEntry, change `.jsonl` → `.db` in path helpers
- `cmd/log.go` — Use `db.Insert`, add `--out-file` flag
- `cmd/new.go` — Create `.db` with schema instead of empty `.jsonl`
- `cmd/switch.go` — Check `.db` instead of `.jsonl`
- `cmd/show.go` — Use `db.LoadAll` / `db.LoadByDate`
- `cmd/search.go` — Use `db.Search`
- `cmd/tail.go` — Use `db.Tail` / `db.TailAfter` for follow
- `cmd/list.go` — Glob `.db`, use `db.Count`
- `cmd/clear.go` — Use `db.Clear`
- `cmd/tag.go` — Use `db.LoadAll` for `listTags()`
- `cmd/timeline.go` — Use `db.LoadAll`
- `cmd/stats.go` — Use `db.LoadAll`
- `cmd/targets.go` — Use `db.LoadAll`
- `cmd/export.go` — Use `db.LoadAll`
- `cmd/log_test.go` — Update to verify via `db.Open` + `db.LoadAll` instead of reading JSONL
- `internal/logfile/logfile_test.go` — Remove `TestLoadEntries_*`, `TestCountEntries_*`; update `TestEngagementName` for `.db`
- `hook.zsh` — Replace JSONL append with `rtlog log --out-file`
- `hook.bash` — Same
- `hook-noninteractive.zsh` — Replace JSONL append with `rtlog log`
- `hook-noninteractive.bash` — Same

---

## Chunk 1: Foundation

### Task 1: Add SQLite Dependency

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add modernc.org/sqlite**

```bash
cd /home/cyb3r/rtlog-go && go get modernc.org/sqlite
```

- [ ] **Step 2: Verify build still works**

```bash
CGO_ENABLED=0 go build -o /dev/null .
```
Expected: builds successfully with no CGo errors

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add modernc.org/sqlite for CGO-free SQLite support"
```

---

### Task 2: Create internal/db Package

**Files:**
- Create: `internal/db/db.go`
- Create: `internal/db/db_test.go`

- [ ] **Step 1: Write failing tests for Open and Insert**

Create `internal/db/db_test.go`:

```go
package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cyb33rr/rtlog/internal/logfile"
)

func tmpDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

func sampleEntry() logfile.LogEntry {
	return logfile.LogEntry{
		Ts:    "2026-03-16T10:00:00Z",
		Epoch: 1773928800,
		User:  "cyb3r",
		Host:  "kali",
		TTY:   "/dev/pts/1",
		Cwd:   "/home/cyb3r",
		Tool:  "nmap",
		Cmd:   "nmap -sV 10.10.10.5",
		Exit:  0,
		Dur:   12.3,
		Tag:   "recon",
		Note:  "initial scan",
		Out:   "open ports: 22, 80",
	}
}

func TestOpenCreatesDB(t *testing.T) {
	dir := tmpDir(t)
	d, err := Open(dir, "test-engagement")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer d.Close()

	dbPath := filepath.Join(dir, "test-engagement.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("DB file not created")
	}
}

func TestOpenSetsWAL(t *testing.T) {
	dir := tmpDir(t)
	d, err := Open(dir, "test-wal")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer d.Close()

	var mode string
	d.conn.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if mode != "wal" {
		t.Fatalf("expected WAL mode, got %q", mode)
	}
}

func TestOpenSetsUserVersion(t *testing.T) {
	dir := tmpDir(t)
	d, err := Open(dir, "test-version")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer d.Close()

	var ver int
	d.conn.QueryRow("PRAGMA user_version").Scan(&ver)
	if ver != schemaVersion {
		t.Fatalf("expected user_version=%d, got %d", schemaVersion, ver)
	}
}

func TestInsertAndLoadAll(t *testing.T) {
	dir := tmpDir(t)
	d, err := Open(dir, "test-insert")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer d.Close()

	e := sampleEntry()
	if err := d.Insert(e); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	entries, err := d.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	got := entries[0]
	if got.Cmd != e.Cmd {
		t.Errorf("Cmd = %q, want %q", got.Cmd, e.Cmd)
	}
	if got.Tool != e.Tool {
		t.Errorf("Tool = %q, want %q", got.Tool, e.Tool)
	}
	if got.ID == 0 {
		t.Error("expected non-zero ID")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/db/ -v
```
Expected: compilation error — package `db` does not exist yet

- [ ] **Step 3: Write Open, Insert, LoadAll, Close, and scanEntries**

Create `internal/db/db.go`:

```go
package db

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/cyb33rr/rtlog/internal/logfile"
)

const schemaVersion = 1

const createSQL = `
CREATE TABLE IF NOT EXISTS entries (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    ts       TEXT    NOT NULL,
    epoch    INTEGER NOT NULL,
    user     TEXT    NOT NULL,
    host     TEXT    NOT NULL,
    tty      TEXT    NOT NULL DEFAULT '',
    cwd      TEXT    NOT NULL,
    tool     TEXT    NOT NULL,
    cmd      TEXT    NOT NULL,
    exit     INTEGER NOT NULL DEFAULT 0,
    dur      REAL    NOT NULL DEFAULT 0.0,
    tag      TEXT    NOT NULL DEFAULT '',
    note     TEXT    NOT NULL DEFAULT '',
    out      TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_entries_epoch ON entries(epoch);
CREATE INDEX IF NOT EXISTS idx_entries_tool  ON entries(tool);
CREATE INDEX IF NOT EXISTS idx_entries_tag   ON entries(tag);
`

const selectCols = "id, ts, epoch, user, host, tty, cwd, tool, cmd, exit, dur, tag, note, out"

// DB wraps a SQLite connection for a single engagement.
type DB struct {
	conn *sql.DB
}

// Open opens or creates a SQLite database for the given engagement.
func Open(dir, engagement string) (*DB, error) {
	path := filepath.Join(dir, engagement+".db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("set WAL: %w", err)
	}

	var ver int
	conn.QueryRow("PRAGMA user_version").Scan(&ver)
	if ver == 0 {
		if _, err := conn.Exec(createSQL); err != nil {
			conn.Close()
			return nil, fmt.Errorf("create schema: %w", err)
		}
		if _, err := conn.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
			conn.Close()
			return nil, fmt.Errorf("set version: %w", err)
		}
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.conn.Close()
}

// Insert adds a log entry to the database.
func (d *DB) Insert(e logfile.LogEntry) error {
	_, err := d.conn.Exec(
		`INSERT INTO entries (ts, epoch, user, host, tty, cwd, tool, cmd, exit, dur, tag, note, out)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Ts, e.Epoch, e.User, e.Host, e.TTY, e.Cwd, e.Tool, e.Cmd, e.Exit, e.Dur, e.Tag, e.Note, e.Out,
	)
	return err
}

func (d *DB) scanEntries(rows *sql.Rows) ([]logfile.LogEntry, error) {
	var entries []logfile.LogEntry
	for rows.Next() {
		var e logfile.LogEntry
		if err := rows.Scan(
			&e.ID, &e.Ts, &e.Epoch, &e.User, &e.Host, &e.TTY,
			&e.Cwd, &e.Tool, &e.Cmd, &e.Exit, &e.Dur,
			&e.Tag, &e.Note, &e.Out,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// LoadAll returns all entries ordered by id.
func (d *DB) LoadAll() ([]logfile.LogEntry, error) {
	rows, err := d.conn.Query("SELECT " + selectCols + " FROM entries ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanEntries(rows)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/db/ -v -run "TestOpen|TestInsert"
```
Expected: all 4 tests PASS

- [ ] **Step 5: Add ID field to LogEntry**

In `internal/logfile/logfile.go`, add `ID` field at line 19 (top of struct):

```go
type LogEntry struct {
	ID    int64   `json:"-"`
	Ts    string  `json:"ts"`
```

The `json:"-"` tag ensures it doesn't appear in JSON marshaling (export, JSONL).

- [ ] **Step 6: Run all existing tests to verify no regressions**

```bash
go test ./...
```
Expected: all tests PASS (ID field is ignored in JSON)

- [ ] **Step 7: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go internal/logfile/logfile.go
git commit -m "feat: add internal/db package with SQLite Open, Insert, LoadAll"
```

---

### Task 3: Add Remaining db Query Methods with Tests

**Files:**
- Modify: `internal/db/db.go`
- Modify: `internal/db/db_test.go`

- [ ] **Step 1: Write failing tests for LoadByDate, Search, Tail, TailAfter, Count, Clear**

Append to `internal/db/db_test.go`:

```go
func insertN(t *testing.T, d *DB, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		e := sampleEntry()
		e.Epoch = 1773928800 + int64(i)
		e.Ts = fmt.Sprintf("2026-03-16T10:%02d:%02dZ", i/60, i%60)
		e.Cmd = fmt.Sprintf("nmap -sV 10.10.10.%d", i+1)
		if err := d.Insert(e); err != nil {
			t.Fatalf("Insert %d failed: %v", i, err)
		}
	}
}

func TestLoadByDate(t *testing.T) {
	dir := tmpDir(t)
	d, err := Open(dir, "test-date")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	e1 := sampleEntry()
	e1.Ts = "2026-03-16T10:00:00Z"
	e2 := sampleEntry()
	e2.Ts = "2026-03-17T10:00:00Z"
	d.Insert(e1)
	d.Insert(e2)

	entries, err := d.LoadByDate("2026-03-16")
	if err != nil {
		t.Fatalf("LoadByDate: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestSearch(t *testing.T) {
	dir := tmpDir(t)
	d, err := Open(dir, "test-search")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	e1 := sampleEntry()
	e1.Cmd = "nmap -sV 10.10.10.5"
	e2 := sampleEntry()
	e2.Cmd = "gobuster dir -u http://target.com"
	e2.Tool = "gobuster"
	d.Insert(e1)
	d.Insert(e2)

	results, err := d.Search("10.10.10.5")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Tool != "nmap" {
		t.Errorf("expected nmap, got %s", results[0].Tool)
	}
}

func TestSearchUser(t *testing.T) {
	dir := tmpDir(t)
	d, err := Open(dir, "test-search-user")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	e := sampleEntry()
	e.User = "operator1"
	d.Insert(e)

	results, err := d.Search("operator1")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestTail(t *testing.T) {
	dir := tmpDir(t)
	d, err := Open(dir, "test-tail")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	insertN(t, d, 10)

	entries, err := d.Tail(3)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// Should be in chronological order (oldest first)
	if entries[0].ID >= entries[1].ID {
		t.Error("expected chronological order")
	}
}

func TestTailAfter(t *testing.T) {
	dir := tmpDir(t)
	d, err := Open(dir, "test-tailafter")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	insertN(t, d, 5)

	all, _ := d.LoadAll()
	lastID := all[2].ID // Get ID of 3rd entry

	entries, err := d.TailAfter(lastID)
	if err != nil {
		t.Fatalf("TailAfter: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after id=%d, got %d", lastID, len(entries))
	}
}

func TestCount(t *testing.T) {
	dir := tmpDir(t)
	d, err := Open(dir, "test-count")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	insertN(t, d, 7)

	count, err := d.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 7 {
		t.Fatalf("expected 7, got %d", count)
	}
}

func TestClear(t *testing.T) {
	dir := tmpDir(t)
	d, err := Open(dir, "test-clear")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	insertN(t, d, 5)
	if err := d.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	count, err := d.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 after clear, got %d", count)
	}
}

func TestReopenExistingDB(t *testing.T) {
	dir := tmpDir(t)

	d1, _ := Open(dir, "test-reopen")
	d1.Insert(sampleEntry())
	d1.Close()

	d2, err := Open(dir, "test-reopen")
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer d2.Close()

	entries, _ := d2.LoadAll()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after reopen, got %d", len(entries))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/db/ -v -run "TestLoadByDate|TestSearch|TestTail|TestCount|TestClear|TestReopen"
```
Expected: FAIL — methods not defined

- [ ] **Step 3: Implement remaining methods**

Append to `internal/db/db.go`:

```go
// LoadByDate returns entries whose ts starts with the given date string (YYYY-MM-DD).
func (d *DB) LoadByDate(dateStr string) ([]logfile.LogEntry, error) {
	rows, err := d.conn.Query(
		"SELECT "+selectCols+" FROM entries WHERE ts LIKE ? ORDER BY id",
		dateStr+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanEntries(rows)
}

// Search returns entries matching keyword across cmd, tool, cwd, tag, note, user, host.
func (d *DB) Search(keyword string) ([]logfile.LogEntry, error) {
	like := "%" + keyword + "%"
	rows, err := d.conn.Query(
		"SELECT "+selectCols+" FROM entries WHERE "+
			"cmd LIKE ? OR tool LIKE ? OR cwd LIKE ? OR "+
			"tag LIKE ? OR note LIKE ? OR user LIKE ? OR host LIKE ? "+
			"ORDER BY id",
		like, like, like, like, like, like, like,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanEntries(rows)
}

// Tail returns the last n entries in chronological order.
func (d *DB) Tail(n int) ([]logfile.LogEntry, error) {
	rows, err := d.conn.Query(
		"SELECT "+selectCols+" FROM (SELECT "+selectCols+" FROM entries ORDER BY id DESC LIMIT ?) ORDER BY id",
		n,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanEntries(rows)
}

// TailAfter returns entries with id > afterID in chronological order.
func (d *DB) TailAfter(afterID int64) ([]logfile.LogEntry, error) {
	rows, err := d.conn.Query(
		"SELECT "+selectCols+" FROM entries WHERE id > ? ORDER BY id",
		afterID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanEntries(rows)
}

// Count returns the total number of entries.
func (d *DB) Count() (int, error) {
	var n int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM entries").Scan(&n)
	return n, err
}

// Clear deletes all entries.
func (d *DB) Clear() error {
	_, err := d.conn.Exec("DELETE FROM entries")
	return err
}
```

- [ ] **Step 4: Run all db tests**

```bash
go test ./internal/db/ -v
```
Expected: all tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go
git commit -m "feat: add db.Search, LoadByDate, Tail, TailAfter, Count, Clear"
```

---

### Task 4: Update internal/logfile Path Helpers

**Files:**
- Modify: `internal/logfile/logfile.go`

**Note:** `logfile` cannot import `internal/db` because `db` already imports `logfile` (for `LogEntry`). This means `CountEntries` must be removed from logfile — `list.go` will call `db.Count` directly in Task 11.

- [ ] **Step 1: Update AvailableEngagements to glob .db**

In `internal/logfile/logfile.go`, change:
```go
matches, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
```
to:
```go
matches, err := filepath.Glob(filepath.Join(dir, "*.db"))
```

Also update the function's doc comment from `.jsonl` to `.db`.

- [ ] **Step 2: Update GetLogPath to use .db extension**

In `internal/logfile/logfile.go`, change line 178:
```go
candidate := filepath.Join(dir, engagement+".jsonl")
```
to:
```go
candidate := filepath.Join(dir, engagement+".db")
```

Also update the error message at line 197:
```go
fmt.Fprintf(os.Stderr, "No .jsonl log files found in %s\n", dir)
```
to:
```go
fmt.Fprintf(os.Stderr, "No log databases found in %s\n", dir)
```

And update the function's doc comment from `.jsonl` to `.db`.

- [ ] **Step 3: Update EngagementName to strip .db**

In `internal/logfile/logfile.go`, change:
```go
return strings.TrimSuffix(filepath.Base(path), ".jsonl")
```
to:
```go
return strings.TrimSuffix(filepath.Base(path), ".db")
```

And update the doc comment from `.jsonl` to `.db`.

- [ ] **Step 4: Remove CountEntries function**

Delete the `CountEntries` function (lines 206-222). It would need to import `internal/db` which creates a circular import. The `list.go` command will call `db.Open` + `db.Count` directly (Task 11).

Also remove `LoadEntries` (lines 70-119) and `parseDate` (lines 122-134) — replaced by `db.LoadAll()` and `db.LoadByDate()`. Commands will call `db` directly.

Remove the `bufio` and `timeutil` imports if no longer used after these deletions. Keep `encoding/json` if `ToMap` or other functions still need it.

- [ ] **Step 5: Verify logfile package compiles**

```bash
go build ./internal/logfile/
```
Expected: compiles. The `cmd/` package will fail to build until Tasks 8-12 migrate callers — that's expected.

**Important:** Do NOT commit yet. We'll commit this together with the cmd/ changes in Task 12 to keep the tree buildable at every commit.

- [ ] **Step 6: Commit (deferred to Task 12)**

This task's changes are committed together with all cmd/ migrations in Task 12, Step 9 to avoid a broken build state.

---

## Chunk 2: Write Path

### Task 5: Update cmd/new.go

**Files:**
- Modify: `cmd/new.go:30-45`

- [ ] **Step 1: Replace file creation with db.Open**

Replace lines 30-45 in `cmd/new.go`. The current code creates an empty `.jsonl` file with `O_CREATE|O_EXCL`. Replace with:

```go
dir := logfile.LogDir()
if err := os.MkdirAll(dir, 0o755); err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}

dbPath := filepath.Join(dir, name+".db")
if _, err := os.Stat(dbPath); err == nil {
    fmt.Fprintf(os.Stderr, "error: engagement %q already exists\n", name)
    os.Exit(1)
}

d, err := db.Open(dir, name)
if err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}
d.Close()
```

Also update the success message `fmt.Printf` (around line 56) to reference the `.db` path instead of the old `.jsonl` path. And update the state update block to use `name` for the engagement key (same as before).

Add `"github.com/cyb33rr/rtlog/internal/db"` to imports. Remove unused imports if any (e.g., `os.O_CREATE` related imports).

- [ ] **Step 2: Verify build**

```bash
go build ./cmd/... 2>&1 | head -20
```

- [ ] **Step 3: Commit**

```bash
git add cmd/new.go
git commit -m "feat: rtlog new creates .db with SQLite schema"
```

---

### Task 6: Update cmd/log.go

**Files:**
- Modify: `cmd/log.go:18-175`

- [ ] **Step 1: Add --out-file and --tty flags**

Add new flag variables alongside existing ones (around line 27):
```go
logCmdOutFile string
logCmdTTY     string
```

In the `init()` function (around line 55), add:
```go
logCmd.Flags().StringVar(&logCmdOutFile, "out-file", "", "read output from file instead of --out")
logCmd.Flags().StringVar(&logCmdTTY, "tty", "", "TTY device (default: auto-detect or 'noninteractive')")
```

- [ ] **Step 2: Update TTY field in entry construction**

Change the hardcoded `TTY: "noninteractive"` (line 135) to:
```go
TTY: logCmdTTY,
```

And after the entry is constructed, default TTY if not provided:
```go
if entry.TTY == "" {
    entry.TTY = "noninteractive"
}
```

This allows interactive hooks to pass the actual TTY (e.g., `/dev/pts/1`) via `--tty`.

- [ ] **Step 3: Add out-file reading logic**

After the entry is constructed (around line 144), add:
```go
if logCmdOutFile != "" {
    data, err := os.ReadFile(logCmdOutFile)
    if err != nil {
        fmt.Fprintf(os.Stderr, "warning: could not read out-file: %v\n", err)
    } else {
        entry.Out = string(data)
    }
    os.Remove(logCmdOutFile)
}
```

- [ ] **Step 4: Replace JSON/file-append with db.Insert**

Replace the JSON marshaling and file writing block (lines 146-167) with:

```go
logDir := os.Getenv("RTLOG_DIR")
if logDir == "" {
    logDir = logfile.LogDir()
}
os.MkdirAll(logDir, 0755)
d, err := db.Open(logDir, st[state.KeyEngagement])
if err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}
defer d.Close()

if err := d.Insert(entry); err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}
```

Add `"github.com/cyb33rr/rtlog/internal/db"` to imports. Remove `"encoding/json"` if no longer used. Keep `"path/filepath"` if still used.

- [ ] **Step 5: Update Long description**

Change the Long description (line 32) from mentioning "JSONL file" to "SQLite database".

- [ ] **Step 6: Update cmd/log_test.go**

The existing tests read `test-eng.jsonl` and JSON-unmarshal. Since `log.go` now writes to SQLite, tests must verify via `db.Open` + `db.LoadAll`:

- `TestLogWritesEntry`: Replace `os.ReadFile(filepath.Join(dir, "test-eng.jsonl"))` + `json.Unmarshal` with `db.Open(dir, "test-eng")` + `d.LoadAll()`. Verify entry fields match.
- `TestLogSkipsUnmatchedTool`: Replace `os.ReadFile(filepath.Join(dir, "test-eng.jsonl"))` check with `db.Open(dir, "test-eng")` + `d.Count()` == 0 (or db file doesn't exist).

Add `"github.com/cyb33rr/rtlog/internal/db"` to test imports.

- [ ] **Step 7: Verify build and tests**

```bash
go build -o /dev/null . && go test ./cmd/ -v -run "TestLog"
```

- [ ] **Step 8: Commit**

```bash
git add cmd/log.go cmd/log_test.go
git commit -m "feat: rtlog log writes to SQLite, add --out-file and --tty flags"
```

---

### Task 7: Update cmd/switch.go

**Files:**
- Modify: `cmd/switch.go`

- [ ] **Step 1: Change .jsonl to .db in path construction**

`switch.go` constructs the path directly at line 28. Change:
```go
logPath := filepath.Join(dir, name+".jsonl")
```
to:
```go
logPath := filepath.Join(dir, name+".db")
```

- [ ] **Step 2: Verify build**

```bash
go build -o /dev/null .
```

- [ ] **Step 3: Commit**

```bash
git add cmd/switch.go
git commit -m "fix: switch checks .db instead of .jsonl"
```

---

## Chunk 3: Read Path

### Task 8: Update cmd/show.go

**Files:**
- Modify: `cmd/show.go:17-87`

- [ ] **Step 1: Replace LoadEntries with db calls**

Replace the entry loading block (around lines 24-42) with:

```go
logPath := logfile.GetLogPath(engagementFlag)
if logPath == "" {
    fmt.Fprintln(os.Stderr, "error: no active engagement (use 'rtlog new' or 'rtlog switch')")
    os.Exit(1)
}

dir := filepath.Dir(logPath)
eng := logfile.EngagementName(logPath)
d, err := db.Open(dir, eng)
if err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}
defer d.Close()

var entries []logfile.LogEntry
if showDate != "" {
    entries, err = d.LoadByDate(showDate)
} else if showToday {
    today := time.Now().UTC().Format("2006-01-02")
    entries, err = d.LoadByDate(today)
} else {
    entries, err = d.LoadAll()
}
if err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}
```

Add imports for `"github.com/cyb33rr/rtlog/internal/db"` and `"path/filepath"`. Remove `"time"` import if `time` package is already imported (keep it for `showToday`).

- [ ] **Step 2: Verify build**

```bash
go build -o /dev/null .
```

- [ ] **Step 3: Commit**

```bash
git add cmd/show.go
git commit -m "feat: show command reads from SQLite"
```

---

### Task 9: Update cmd/search.go

**Files:**
- Modify: `cmd/search.go:13-62`

- [ ] **Step 1: Replace LoadEntries and regex matching with db.Search**

The current search loads all entries and uses regex matching in Go. We keep the regex matching for highlight but use `db.Search` for the initial filter. Replace the entry loading and filtering block with:

```go
logPath := logfile.GetLogPath(engagementFlag)
if logPath == "" {
    fmt.Fprintln(os.Stderr, "error: no active engagement")
    os.Exit(1)
}

dir := filepath.Dir(logPath)
eng := logfile.EngagementName(logPath)
d, err := db.Open(dir, eng)
if err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}
defer d.Close()

entries, err := d.Search(keyword)
if err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}
```

Keep the existing regex pattern compilation (for highlighting) and the display loop. Only replace the loading/filtering logic.

Add imports for `"github.com/cyb33rr/rtlog/internal/db"` and `"path/filepath"`.

- [ ] **Step 2: Verify build**

```bash
go build -o /dev/null .
```

- [ ] **Step 3: Commit**

```bash
git add cmd/search.go
git commit -m "feat: search command uses SQLite LIKE queries"
```

---

### Task 10: Update cmd/tail.go

**Files:**
- Modify: `cmd/tail.go:21-117`

- [ ] **Step 1: Replace initial load with db.Tail**

Replace the initial entry loading block with:

```go
logPath := logfile.GetLogPath(engagementFlag)
if logPath == "" {
    fmt.Fprintln(os.Stderr, "error: no active engagement")
    os.Exit(1)
}

dir := filepath.Dir(logPath)
eng := logfile.EngagementName(logPath)

d, err := db.Open(dir, eng)
if err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}

entries, err := d.Tail(tailN)
if err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}

for _, e := range entries {
    fmt.Println(display.FmtEntry(logfile.ToMap(e), 0, 0))
}

var lastID int64
if len(entries) > 0 {
    lastID = entries[len(entries)-1].ID
}
d.Close()
```

- [ ] **Step 2: Replace follow loop with db.TailAfter polling**

Replace the follow loop (lines 61-115) with:

```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

for {
    select {
    case <-sigCh:
        return
    default:
    }

    time.Sleep(500 * time.Millisecond)

    d, err = db.Open(dir, eng)
    if err != nil {
        continue
    }

    newEntries, err := d.TailAfter(lastID)
    d.Close()
    if err != nil {
        continue
    }

    for _, e := range newEntries {
        fmt.Println(display.FmtEntry(logfile.ToMap(e), 0, 0))
        lastID = e.ID
    }
}
```

Add imports: `"github.com/cyb33rr/rtlog/internal/db"`, `"path/filepath"`. Keep `"os/signal"`, `"syscall"`, `"time"`. Remove `"bufio"`, `"io"` if no longer used.

- [ ] **Step 3: Verify build**

```bash
go build -o /dev/null .
```

- [ ] **Step 4: Commit**

```bash
git add cmd/tail.go
git commit -m "feat: tail command reads from SQLite with polling follow"
```

---

### Task 11: Update cmd/list.go and cmd/clear.go

**Files:**
- Modify: `cmd/list.go:13-38`
- Modify: `cmd/clear.go:19-62`

- [ ] **Step 1: Update list.go**

`list.go` uses `logfile.AvailableEngagements()` (now returns `.db` paths) and `logfile.CountEntries()` (removed in Task 4). Replace `logfile.CountEntries(path)` with direct `db.Open` + `db.Count`:

```go
dir := filepath.Dir(path)
eng := logfile.EngagementName(path)
d, err := db.Open(dir, eng)
count := 0
if err == nil {
    count, _ = d.Count()
    d.Close()
}
```

Add imports: `"github.com/cyb33rr/rtlog/internal/db"`, `"path/filepath"`.

Check for and update any `.jsonl` string literals.

- [ ] **Step 2: Update clear.go**

Replace the file truncation (line 55) with:

```go
dir := filepath.Dir(logPath)
eng := logfile.EngagementName(logPath)
d, err := db.Open(dir, eng)
if err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}
if err := d.Clear(); err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}
d.Close()
```

Add imports: `"github.com/cyb33rr/rtlog/internal/db"`, `"path/filepath"`.

- [ ] **Step 3: Verify build**

```bash
go build -o /dev/null .
```

- [ ] **Step 4: Commit**

```bash
git add cmd/list.go cmd/clear.go
git commit -m "feat: list and clear commands use SQLite"
```

---

### Task 12: Update Remaining Read Commands

**Files:**
- Modify: `cmd/timeline.go:21-105`
- Modify: `cmd/stats.go:27-148`
- Modify: `cmd/targets.go:39-103`
- Modify: `cmd/export.go:14-57`

All four commands follow the same pattern: load all entries, then process in Go. Replace the `logfile.LoadEntries` call in each with:

- [ ] **Step 1: Create a shared helper for opening db from engagement flag**

Since every command repeats the same open pattern, add a helper to `cmd/root.go`:

```go
func openEngagementDB() (*db.DB, error) {
	logPath := logfile.GetLogPath(engagementFlag)
	if logPath == "" {
		return nil, fmt.Errorf("no active engagement (use 'rtlog new' or 'rtlog switch')")
	}
	dir := filepath.Dir(logPath)
	eng := logfile.EngagementName(logPath)
	return db.Open(dir, eng)
}
```

Add `"github.com/cyb33rr/rtlog/internal/db"` and `"path/filepath"` to root.go imports.

Also update the Long description in `rootCmd` (line 24) from mentioning "JSONL" / `.jsonl` to "SQLite" / `.db`.

- [ ] **Step 2: Update tag.go**

`cmd/tag.go` has a `listTags()` function that calls `logfile.LoadEntries`. Replace with:
```go
d, err := openEngagementDB()
if err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}
defer d.Close()

entries, err := d.LoadAll()
if err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}
```

- [ ] **Step 3: Update timeline.go**

Replace the entry loading block with:
```go
d, err := openEngagementDB()
if err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}
defer d.Close()

entries, err := d.LoadAll()
if err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}
```

- [ ] **Step 4: Update stats.go**

Same pattern as timeline.go — replace `logfile.LoadEntries` with `openEngagementDB()` + `d.LoadAll()`.

- [ ] **Step 5: Update targets.go**

Same pattern — replace the entry loading block in `runTargets`.

- [ ] **Step 6: Update export.go**

Same pattern — replace the entry loading block.

- [ ] **Step 7: Update clear.go CountEntries call**

`clear.go` line 32 calls `logfile.CountEntries(path)` for the pre-clear count. Replace with:
```go
d, err := openEngagementDB()
if err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}
count, _ := d.Count()
```
Then use `count` in the confirmation prompt. Keep the `d.Clear()` from Task 11.

- [ ] **Step 8: Refactor show.go, search.go to use openEngagementDB()**

Go back and refactor the earlier commands to use the helper where applicable. Tail is slightly different (needs dir/eng for polling) so it keeps its own open logic.

- [ ] **Step 9: Update internal/logfile/logfile_test.go**

- Delete `TestLoadEntries_*` tests (4 tests for a removed function)
- Delete `TestCountEntries_LargeLine` test (removed function)
- Update `TestEngagementName` to expect `.db` suffix stripping instead of `.jsonl`

- [ ] **Step 10: Verify full build**

```bash
go build -o /dev/null .
```
Expected: compiles with no errors

- [ ] **Step 11: Run all tests**

```bash
go test ./...
```
Expected: all tests PASS

- [ ] **Step 12: Commit (includes deferred logfile.go changes from Task 4)**

```bash
git add internal/logfile/logfile.go internal/logfile/logfile_test.go cmd/root.go cmd/tag.go cmd/timeline.go cmd/stats.go cmd/targets.go cmd/export.go cmd/show.go cmd/search.go cmd/clear.go cmd/list.go
git commit -m "feat: all read commands use SQLite, remove JSONL loading from logfile"
```

This commit bundles the logfile.go changes (Task 4) with all cmd/ migrations and test updates to keep the tree buildable at every commit.

---

## Chunk 4: Migration & Hooks

### Task 13: Create rtlog import Command

**Files:**
- Create: `cmd/importcmd.go`

- [ ] **Step 1: Implement import command**

Create `cmd/importcmd.go`:

```go
package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/db"
	"github.com/cyb33rr/rtlog/internal/logfile"
)

var importCmd = &cobra.Command{
	Use:   "import <file.jsonl> [file2.jsonl ...]",
	Short: "Import JSONL log files into SQLite databases",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		for _, path := range args {
			importFile(path)
		}
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
}

func importFile(path string) {
	base := filepath.Base(path)
	eng := strings.TrimSuffix(base, ".jsonl")
	if eng == base {
		fmt.Fprintf(os.Stderr, "skip: %s is not a .jsonl file\n", path)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
		return
	}
	defer f.Close()

	dir := logfile.LogDir()
	d, err := db.Open(dir, eng)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", eng, err)
		return
	}
	defer d.Close()

	// Load existing entries for dedup
	existing := make(map[string]bool)
	entries, _ := d.LoadAll()
	for _, e := range entries {
		key := dedupKey(e)
		existing[key] = true
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	var imported, skipped, malformed int

	for scanner.Scan() {
		line := scanner.Bytes()
		var e logfile.LogEntry
		if err := json.Unmarshal(line, &e); err != nil {
			malformed++
			continue
		}

		key := dedupKey(e)
		if existing[key] {
			skipped++
			continue
		}

		if err := d.Insert(e); err != nil {
			fmt.Fprintf(os.Stderr, "warning: insert failed: %v\n", err)
			continue
		}
		existing[key] = true
		imported++
	}

	fmt.Printf("Imported %d entries into engagement %q", imported, eng)
	if skipped > 0 {
		fmt.Printf(" (%d duplicates skipped)", skipped)
	}
	if malformed > 0 {
		fmt.Printf(" (%d malformed lines skipped)", malformed)
	}
	fmt.Println()
}

func dedupKey(e logfile.LogEntry) string {
	return fmt.Sprintf("%d|%s|%s|%s", e.Epoch, e.Cmd, e.Tool, e.Cwd)
}
```

- [ ] **Step 2: Verify build**

```bash
go build -o /dev/null .
```

- [ ] **Step 3: Write an integration test**

Create a temp JSONL file, run import, verify entries are in the DB. Add to `internal/db/db_test.go`:

```go
func TestImportDedup(t *testing.T) {
	dir := tmpDir(t)
	d, err := Open(dir, "test-dedup")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	e := sampleEntry()
	d.Insert(e)
	d.Insert(e) // Same entry twice — no dedup at db level

	count, _ := d.Count()
	if count != 2 {
		t.Fatalf("expected 2 (db allows dupes), got %d", count)
	}
	d.Close()
}
```

- [ ] **Step 4: Commit**

```bash
git add cmd/importcmd.go internal/db/db_test.go
git commit -m "feat: add rtlog import command for JSONL-to-SQLite migration"
```

---

### Task 14: Update Shell Hooks

**Files:**
- Modify: `hook.zsh:252-256` (JSONL write block)
- Modify: `hook.bash:252-262` (JSONL write block)
- Modify: `hook-noninteractive.zsh:190-193` (JSONL write block)
- Modify: `hook-noninteractive.bash:183-194` (JSONL write block)

- [ ] **Step 1: Update hook.zsh — replace JSONL append with rtlog log**

In `hook.zsh`, find the `_rtlog_precmd()` function's JSONL write block (around lines 252-256). Replace the `printf ... >> "$RTLOG_DIR/logs/${RTLOG_ENGAGEMENT}.jsonl"` with:

```zsh
# Write to SQLite via rtlog log
local _out_args=()
if [[ -n "$_rtlog_tmpfile" && -f "$_rtlog_tmpfile" ]]; then
    _out_args=(--out-file "$_rtlog_tmpfile")
fi

rtlog log \
    --cmd "$_rtlog_last_cmd" \
    --tool "$_rtlog_last_tool" \
    --exit "$_rtlog_last_rc" \
    --dur "$_dur" \
    --cwd "$_rtlog_last_cwd" \
    --tty "$_rtlog_tty" \
    "${_out_args[@]}" 2>/dev/null
```

**Important:** Do NOT pass `--tag` or `--note` flags. `cmd/log.go` reads tag and note from the state file automatically. Passing them explicitly would cause `cmd.Flags().Changed()` to return true, which prevents the one-shot note from being cleared. The current hooks clear the note themselves after writing JSONL — with `rtlog log`, this is handled by `cmd/log.go` (lines 169-174) but only when the flag is NOT explicitly set.

Remove the JSON escaping calls (`_rtlog_json_escape`) that were only used for building the JSONL JSON string. Since `rtlog log` accepts plain strings as flags, no escaping is needed.

- [ ] **Step 2: Update hook.bash — same pattern**

In `hook.bash`, replace the JSONL write block in `_rtlog_precmd()` with the same `rtlog log` call, using bash syntax:

```bash
# Write to SQLite via rtlog log
local _out_args=()
if [[ -n "$_rtlog_tmpfile" && -f "$_rtlog_tmpfile" ]]; then
    _out_args=(--out-file "$_rtlog_tmpfile")
fi

rtlog log \
    --cmd "$_rtlog_last_cmd" \
    --tool "$_rtlog_last_tool" \
    --exit "$_rtlog_last_rc" \
    --dur "$_dur" \
    --cwd "$_rtlog_last_cwd" \
    --tty "$_rtlog_tty" \
    "${_out_args[@]}" 2>/dev/null
```

**Same note about --tag/--note:** Do NOT pass them. Let `cmd/log.go` read from state.

Also remove the one-shot note clearing block from the hook (the hooks currently clear the note after JSONL write; `cmd/log.go` now handles this).

- [ ] **Step 3: Update hook-noninteractive.zsh**

In `_rtlog_ni_exit_handler()`, replace the JSONL write block (around lines 190-193) with:

```zsh
local _out_args=()
if [[ -n "$_rtlog_ni_outfile" && -f "$_rtlog_ni_outfile" ]]; then
    _out_args=(--out-file "$_rtlog_ni_outfile")
fi

rtlog log \
    --cmd "$_rtlog_ni_cmd" \
    --tool "$_rtlog_ni_tool" \
    --exit "$_rtlog_ni_rc" \
    --dur "$_dur" \
    --cwd "$_rtlog_ni_cwd" \
    "${_out_args[@]}" 2>/dev/null
```

No `--tag`, `--note`, or `--tty` needed. `cmd/log.go` reads tag/note from state, and defaults TTY to `"noninteractive"` when not provided.

Also remove the one-shot note clearing block (if present) — `cmd/log.go` handles this.

- [ ] **Step 4: Update hook-noninteractive.bash**

Same pattern as step 3 but for the bash non-interactive hook.

- [ ] **Step 5: Remove dead JSONL-specific code from hooks**

In all 4 hooks, the `_rtlog_json_escape` function and the JSON field construction variables are no longer needed for building JSONL entries. Remove them if they are not used elsewhere in the hook. Verify by checking if any remaining code calls `_rtlog_json_escape`.

- [ ] **Step 6: Commit**

```bash
git add hook.zsh hook.bash hook-noninteractive.zsh hook-noninteractive.bash
git commit -m "feat: shell hooks write via rtlog log instead of direct JSONL"
```

---

### Task 15: Cross-Compilation Verification and Cleanup

**Files:**
- Modify: `Makefile` (no changes expected, just verify)

- [ ] **Step 1: Run full test suite**

```bash
go test ./...
```
Expected: all tests PASS

- [ ] **Step 2: Cross-compile for all 4 targets**

```bash
CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -o /dev/null .
CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -o /dev/null .
CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -o /dev/null .
CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -o /dev/null .
```
Expected: all 4 compile successfully

- [ ] **Step 3: Check binary size**

```bash
go build -o rtlog-sqlite . && ls -lh rtlog-sqlite
```
Note the size increase compared to the current binary.

- [ ] **Step 4: Manual smoke test**

```bash
./rtlog-sqlite new test-sqlite
./rtlog-sqlite log --cmd "nmap -sV 10.10.10.5" --tool nmap --exit 0 --dur 5.2
./rtlog-sqlite show
./rtlog-sqlite search 10.10.10.5
./rtlog-sqlite tail -n 1
./rtlog-sqlite stats
./rtlog-sqlite list
./rtlog-sqlite clear -y
./rtlog-sqlite list
```
Expected: all commands work correctly with SQLite backend

- [ ] **Step 5: Test import with real data (if available)**

```bash
# Only if existing .jsonl files exist:
ls ~/.rt/logs/*.jsonl 2>/dev/null && ./rtlog-sqlite import ~/.rt/logs/*.jsonl
```

- [ ] **Step 6: Clean up temporary binary**

```bash
rm -f rtlog-sqlite
```

- [ ] **Step 7: Remove dead code**

Grep for any remaining `.jsonl` references in Go source:
```bash
grep -r '\.jsonl' --include='*.go' .
```

Remove any leftover references. Also check for unused imports in modified files.

- [ ] **Step 8: Final commit**

```bash
git add -A
git commit -m "chore: cleanup dead JSONL references and verify cross-compilation"
```
