# SQLite Storage for rtlog-go

## Summary

Replace JSONL file storage with SQLite databases using `modernc.org/sqlite` (pure Go, no CGo). Each engagement gets its own `.db` file at `~/.rt/logs/<engagement>.db`. Includes an import command for migrating existing JSONL data.

## Motivation

Future-proof query performance as engagements scale. JSONL requires loading all entries into memory for every query. SQLite provides indexed queries with minimal overhead on the write path (~20-40ms per insert via `rtlog log`, below human perception).

## Constraints

- Must use `modernc.org/sqlite` (pure Go) to preserve `CGO_ENABLED=0` cross-compilation for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
- Binary size increase of ~10-15MB is acceptable
- All existing CLI interfaces and user-facing behavior remain unchanged
- Cross-engagement queries are out of scope for initial implementation

## Storage & Schema

### File Layout

`~/.rt/logs/<engagement>.db` replaces `~/.rt/logs/<engagement>.jsonl`

### Schema

```sql
CREATE TABLE entries (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    ts       TEXT    NOT NULL,  -- ISO 8601 timestamp
    epoch    INTEGER NOT NULL,  -- Unix epoch for fast range queries
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

CREATE INDEX idx_entries_epoch ON entries(epoch);
CREATE INDEX idx_entries_tool  ON entries(tool);
CREATE INDEX idx_entries_tag   ON entries(tag);
```

Same fields as the current `LogEntry` struct. Indexes on `epoch`, `tool`, and `tag` for common query patterns (date filtering, stats by tool, timeline by tag).

### DB Configuration

- WAL mode enabled at DB creation for better concurrent write performance
- DB file created automatically on first write (`rtlog new` or first log entry)

## Write Path

All hooks (interactive and non-interactive) call `rtlog log` to insert into SQLite. This unifies the write path.

### Changes

- **Interactive hooks (hook.zsh, hook.bash):** Replace `echo '{}' >> file.jsonl` with a call to `rtlog log`
- **Non-interactive hooks:** No change to hook code; only the internal implementation of `rtlog log` changes
- **`rtlog log` internals:** Changes from "append JSON line to file" to "open DB, insert row, close"
- **CLI interface unchanged:** `rtlog log --cmd "nmap -sV 10.10.10.5" --tool nmap --exit 0 --dur 12.3`

### DB Lifecycle

- `rtlog log` opens the DB, inserts, closes on every call (no long-lived connections)
- No connection pooling needed — each invocation is a separate process

## Read Path

No changes to CLI interfaces. Internal implementations change from "load all JSONL, filter in Go" to "SQL query, scan rows."

| Command | JSONL (before) | SQLite (after) |
|---------|----------------|----------------|
| `show` | Load all, date filter in Go | `SELECT * WHERE epoch BETWEEN ? AND ?` |
| `tail -n N` | Load all, take last N | `SELECT * ORDER BY id DESC LIMIT ?` |
| `search` | Load all, substring match | `SELECT * WHERE cmd LIKE ? OR tool LIKE ? ...` |
| `timeline` | Load all, group in Go | `SELECT * ORDER BY epoch`, grouping in Go |
| `stats` | Load all, aggregate in Go | Load all, aggregate in Go (unchanged logic) |
| `targets` | Load all, run extraction | Load all, extraction in Go (unchanged logic) |
| `export` | Load all, format | Load all, format (unchanged logic) |
| `list` | Glob `*.jsonl`, count lines | Glob `*.db`, `SELECT COUNT(*) FROM entries` |

Extraction logic (`targets`, `creds`) stays in Go at query time. No schema change needed.

Search uses `LIKE '%keyword%'` across `cmd`, `tool`, `cwd`, `tag`, `note` — case-insensitive for ASCII by default in SQLite. Works for IPs, hostnames, any substring.

## Import & Migration

### New Command: `rtlog import <file.jsonl>`

Imports existing JSONL files into SQLite databases.

### Behavior

- Reads the JSONL file line by line
- Inserts each entry into `~/.rt/logs/<engagement>.db`
- Engagement name derived from filename (e.g., `pentest-acme.jsonl` -> `pentest-acme.db`)
- If `.db` already exists, entries are appended (skipping duplicates based on `epoch + cmd` combo)
- Original `.jsonl` file left untouched (user deletes manually)
- Validates each JSON line before inserting; skips and warns on malformed lines
- Prints progress: `Imported 342 entries into engagement "pentest-acme"`

### Usage

```bash
# Import a specific engagement
rtlog import ~/.rt/logs/pentest-acme.jsonl

# Import all existing engagements
rtlog import ~/.rt/logs/*.jsonl
```

## Package Changes

### New: `internal/db/`

New package encapsulating all SQLite operations:

- `Open(engagement string) (*DB, error)` — opens/creates DB, enables WAL, ensures schema
- `Insert(entry LogEntry) error` — insert a single entry
- `LoadAll() ([]LogEntry, error)` — load all entries (for commands that need full scan)
- `LoadRange(from, to int64) ([]LogEntry, error)` — load by epoch range
- `Search(keyword string) ([]LogEntry, error)` — LIKE search across fields
- `Tail(n int) ([]LogEntry, error)` — last N entries
- `Count() (int, error)` — entry count for `list` command
- `Close() error`

### Modified: `internal/logfile/`

Replace JSONL I/O with calls to `internal/db/`. The `LogEntry` struct stays in this package (or moves to a shared types package). `LoadEntries` and `AvailableEngagements` are updated to use SQLite.

### Modified: `cmd/`

- `log.go` — use `db.Insert` instead of file append
- `show.go`, `tail.go`, `search.go`, `timeline.go`, `stats.go`, `targets.go`, `export.go` — use `db.Load*` / `db.Search` instead of `logfile.LoadEntries`
- `list.go` — glob `*.db` instead of `*.jsonl`, use `db.Count`
- New `import.go` — `rtlog import` command

### Modified: Shell Hooks

- `hook.zsh`, `hook.bash` — replace direct JSONL append with `rtlog log` call

## Out of Scope

- Cross-engagement queries (`--all` flag) — straightforward to add later by iterating over `.db` files
- FTS (full-text search) — `LIKE` is sufficient at realistic engagement sizes
- Normalized schema (separate tables for targets/creds) — extraction stays at query time in Go
- Schema migrations — single table, unlikely to change frequently
