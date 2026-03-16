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
PRAGMA user_version = 1;

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

`PRAGMA user_version = 1` is set at creation time. If the schema changes in the future, the version is checked on open and migrations applied as needed.

### DB Configuration

- WAL mode enabled at DB creation for better concurrent write performance
- `rtlog new` eagerly creates the `.db` file with schema (matches current behavior of creating empty `.jsonl` with `O_CREATE|O_EXCL`). Duplicate engagement names are detected by checking if the `.db` file already exists.

## Write Path

All hooks (interactive and non-interactive) call `rtlog log` to insert into SQLite. This unifies the write path across all four hook files.

### Changes

- **Interactive hooks (hook.zsh, hook.bash):** Replace `printf ... >> file.jsonl` with a call to `rtlog log`
- **Non-interactive hooks (hook-noninteractive.zsh, hook-noninteractive.bash):** Replace `printf ... >> file.jsonl` with a call to `rtlog log`
- **`rtlog log` internals:** Changes from "append JSON line to file" to "open DB, insert row, close"
- **CLI interface unchanged:** `rtlog log --cmd "nmap -sV 10.10.10.5" --tool nmap --exit 0 --dur 12.3`

### Output Capture

Interactive hooks capture stdout/stderr via `tee` + fd redirection. The captured output can be large (megabytes), exceeding `ARG_MAX` if passed via `--out` flag. To handle this:

- Add `--out-file <path>` flag to `rtlog log` — reads output from a temp file instead of a CLI argument
- Hooks write captured output to a temp file (as they already do), then pass the path: `rtlog log --cmd "..." --out-file /tmp/rtlog-out-$$`
- `rtlog log` reads the file contents, inserts into the `out` column, and deletes the temp file
- Falls back to `--out <string>` for small output or programmatic use

### DB Lifecycle

- `rtlog log` opens the DB, inserts, closes on every call (no long-lived connections)
- No connection pooling needed — each invocation is a separate process

### Latency

The write path changes from a zero-overhead `printf >> file` (~0ms) to spawning `rtlog log` + SQLite insert (~20-40ms). This is below human perception (~100ms) and acceptable for the use case.

## Read Path

No changes to CLI interfaces. Internal implementations change from "load all JSONL, filter in Go" to "SQL query, scan rows."

| Command | JSONL (before) | SQLite (after) |
|---------|----------------|----------------|
| `show` | Load all, date filter in Go | `SELECT * WHERE epoch BETWEEN ? AND ?` |
| `tail -n N` | Load all, take last N | `SELECT * ORDER BY id DESC LIMIT ?` |
| `tail -f` (follow) | Seek to end, poll for new lines | Poll with `SELECT * WHERE id > ? ORDER BY id` every 500ms |
| `search` | Load all, regex match | `SELECT * WHERE cmd LIKE ? OR tool LIKE ? ...` |
| `timeline` | Load all, group in Go | `SELECT * ORDER BY epoch`, grouping in Go |
| `stats` | Load all, aggregate in Go | Load all, aggregate in Go (unchanged logic) |
| `targets` | Load all, run extraction | Load all, extraction in Go (unchanged logic) |
| `export` | Load all, format | Load all, format (unchanged logic) |
| `list` | Glob `*.jsonl`, count lines | Glob `*.db`, `SELECT COUNT(*) FROM entries` |
| `clear` | `os.Truncate(path, 0)` | `DELETE FROM entries` (do NOT truncate the `.db` file) |

Extraction logic (`targets`, `creds`) stays in Go at query time. No schema change needed.

### Search

Search uses `LIKE '%keyword%'` across `cmd`, `tool`, `cwd`, `tag`, `note`, `user`, and `host` — matching all fields the current implementation checks. SQLite `LIKE` is case-insensitive for ASCII by default, which is sufficient since search targets are commands, IPs, hostnames, and tool names (all ASCII in practice).

### Date Filtering

The `show --date YYYY-MM-DD` filter currently compares the date portion of the `ts` string (ISO 8601 in UTC). To maintain consistent behavior, the SQLite implementation uses `WHERE ts LIKE 'YYYY-MM-DD%'` rather than epoch range conversion, avoiding timezone ambiguity.

## Import & Migration

### New Command: `rtlog import <file.jsonl> [<file2.jsonl> ...]`

Imports existing JSONL files into SQLite databases. Accepts multiple file arguments (shell glob expands to multiple args).

### Behavior

- Reads each JSONL file line by line
- Inserts each entry into `~/.rt/logs/<engagement>.db`
- Engagement name derived from filename (e.g., `pentest-acme.jsonl` -> `pentest-acme.db`)
- If `.db` already exists, entries are appended (skipping duplicates based on `epoch + cmd + tool + cwd` combo for best-effort dedup)
- Original `.jsonl` files left untouched (user deletes manually)
- Validates each JSON line before inserting; skips and warns on malformed lines
- Each file is imported independently (failure on one file does not abort others)
- Prints progress per file: `Imported 342 entries into engagement "pentest-acme"`

### Usage

```bash
# Import a specific engagement
rtlog import ~/.rt/logs/pentest-acme.jsonl

# Import all existing engagements (shell expands glob)
rtlog import ~/.rt/logs/*.jsonl
```

## Package Changes

### New: `internal/db/`

New package encapsulating all SQLite operations:

- `Open(dir, engagement string) (*DB, error)` — opens/creates DB at `dir/<engagement>.db`, enables WAL, ensures schema, checks `user_version`. Respects `RTLOG_DIR` override via the `dir` parameter passed by callers.
- `Insert(entry LogEntry) error` — insert a single entry
- `LoadAll() ([]LogEntry, error)` — load all entries (for commands that need full scan)
- `LoadRange(from, to int64) ([]LogEntry, error)` — load by epoch range
- `LoadByDate(dateStr string) ([]LogEntry, error)` — load by date string (ts LIKE)
- `Search(keyword string) ([]LogEntry, error)` — LIKE search across cmd, tool, cwd, tag, note, user, host
- `Tail(n int) ([]LogEntry, error)` — last N entries
- `TailAfter(id int64) ([]LogEntry, error)` — entries after a given ID (for live follow)
- `Count() (int, error)` — entry count for `list` command
- `Clear() error` — delete all entries
- `Close() error`

### Modified: `internal/logfile/`

Replace JSONL I/O with calls to `internal/db/`. The `LogEntry` struct stays in this package (or moves to a shared types package). `LoadEntries` and `AvailableEngagements` are updated to use SQLite (glob `*.db` instead of `*.jsonl`).

### Modified: `cmd/`

- `log.go` — use `db.Insert` instead of file append; add `--out-file` flag
- `show.go`, `tail.go`, `search.go`, `timeline.go`, `stats.go`, `targets.go`, `export.go` — use `db.Load*` / `db.Search` instead of `logfile.LoadEntries`
- `tail.go` — live follow uses `db.TailAfter(lastID)` polling loop
- `list.go` — glob `*.db` instead of `*.jsonl`, use `db.Count`
- `clear.go` — use `db.Clear()` instead of `os.Truncate`
- `new.go` — create `.db` file with schema instead of empty `.jsonl`; detect duplicate via file existence check
- `switch.go` — check for `<name>.db` instead of `<name>.jsonl`
- New `import.go` — `rtlog import` command

### Modified: Shell Hooks (all four files)

- `hook.zsh` — replace direct JSONL append with `rtlog log` call; pass captured output via `--out-file`
- `hook.bash` — same
- `hook-noninteractive.zsh` — replace direct JSONL append with `rtlog log` call
- `hook-noninteractive.bash` — same

## Testing Strategy

- Unit tests for `internal/db/` — open, insert, query, search, clear, schema version check
- Integration test: import a known JSONL file, verify all entries read back correctly
- Verify cross-compilation still works: `CGO_ENABLED=0` build for all 4 targets with `modernc.org/sqlite`
- Update existing `cmd/log_test.go` for new write path
- Manual verification of shell hooks in both zsh and bash

## Out of Scope

- Cross-engagement queries (`--all` flag) — straightforward to add later by iterating over `.db` files
- FTS (full-text search) — `LIKE` is sufficient at realistic engagement sizes
- Normalized schema (separate tables for targets/creds) — extraction stays at query time in Go
