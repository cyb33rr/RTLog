# Export Filtering & Search

## Problem

`rtlog export` exports all entries unconditionally. Users need to export subsets filtered by tool, tag, date range, and free-text search. Additionally, `rtlog show` only supports literal substring matching — users need regex search as an option.

## Solution

Add stackable filter flags to `export` and add regex search mode to both `export` and `show`.

## CLI Interface

### Export

```
rtlog export <md|csv|jsonl> [-o file] [filter flags]
```

New flags:

| Flag       | Type   | Description                                     |
|------------|--------|-------------------------------------------------|
| `--tool`   | string | Comma-separated tool names (OR within)           |
| `--tag`    | string | Comma-separated tags (OR within)                 |
| `--date`   | string | Single date YYYY-MM-DD                           |
| `--from`   | string | Range start inclusive YYYY-MM-DD                 |
| `--to`     | string | Range end inclusive YYYY-MM-DD                   |
| `--filter` | string | Literal substring match against cmd, tool, cwd, tag, note |
| `-r`       | string | Regex match against cmd, tool, cwd, tag, note             |

Rules:
- `--date` is mutually exclusive with `--from`/`--to`
- `--from` and `--to` can each be used alone (open-ended range)
- All flags combine with AND logic across different flag types
- `--tool` and `--tag` use OR logic within their comma-separated values
- `--filter` and `-r` can both be used (AND logic) but typically one or the other
- Invalid `-r` regex exits with a clear error message

Examples:
```
rtlog export csv --tool nmap,nxc --tag recon --from 2026-03-15 --to 2026-03-17
rtlog export csv --filter "10.0.0.1"
rtlog export csv -r "10\.0\.0\.\d+"
```

### Show

Two search modes:
- `rtlog show <keyword>` — literal substring match (existing behavior, unchanged)
- `rtlog show -r <pattern>` — regex match

Invalid regex with `-r` prints an error and exits. The `-r` flag and keyword argument are mutually exclusive.

## Architecture

**Hybrid approach:** SQL-level filtering for structured fields (tool, tag, date/epoch) leveraging existing indices. Go-level filtering for `--filter` (substring) and `-r` (regex) using native Go.

### Database Layer (`internal/db/db.go`)

New method:

```go
func (d *DB) LoadFiltered(tools []string, tags []string, date, from, to string) ([]logfile.LogEntry, error)
```

Builds a SQL query dynamically with optional WHERE clauses:

- `tools` non-empty: `WHERE tool IN (?, ?, ...)` (indexed column)
- `tags` non-empty: `AND tag IN (?, ?, ...)` (indexed column)
- `date` non-empty: `AND ts LIKE ?` (date prefix match, e.g. `2026-03-15%`)
- `from` non-empty: `AND epoch >= ?` (epoch of `from` date at 00:00:00 UTC)
- `to` non-empty: `AND epoch <= ?` (epoch of `to` date at 23:59:59 UTC)

When no structured filters are provided, equivalent to `LoadAll()`.

Uses the existing `queryEntries` helper for row scanning.

### Search Filter (`internal/filter/filter.go`)

New package with two functions:

```go
func MatchSubstring(entries []logfile.LogEntry, substr string) []logfile.LogEntry
func MatchRegex(entries []logfile.LogEntry, pattern string) ([]logfile.LogEntry, error)
```

Both test each entry against 5 fields: `cmd`, `tool`, `cwd`, `tag`, `note`. An entry matches if any field matches.

- `MatchSubstring`: case-insensitive substring match (uses `strings.Contains` on lowered fields). Always succeeds.
- `MatchRegex`: compiles `pattern` with `regexp.Compile`. Returns error if pattern is invalid. Returns filtered slice (empty slice, not nil, when no matches).

### Export Command (`cmd/export.go`)

Updated flow:

1. Parse and validate flags
   - Mutual exclusivity: `--date` vs `--from`/`--to`
   - Validate `-r` regex at parse time
   - Split `--tool` and `--tag` on commas
   - Validate date formats for `--date`, `--from`, `--to`
2. Call `db.LoadFiltered(tools, tags, date, from, to)` instead of `db.LoadAll()`
3. If `--filter` is set, pass results through `filter.MatchSubstring()`
4. If `-r` is set, pass results through `filter.MatchRegex()`
5. If zero entries remain: print `"No entries match the given filters"` to stderr and exit (no file created)
6. Otherwise pass to format-specific exporter as before

### Show Command (`cmd/show.go`)

Two search modes:

**Literal mode** (`rtlog show keyword`) — unchanged behavior:
1. Keyword passed to `Search()`/`SearchByDate()` for SQL LIKE matching
2. Highlighting uses `regexp.QuoteMeta(keyword)` as before

**Regex mode** (`rtlog show -r pattern`) — new:
1. Validate regex at parse time; invalid regex prints error and exits
2. DB-level `Search()` still does an initial broad LIKE query (using the raw pattern as a best-effort substring)
3. Results refined with `filter.MatchRegex()` to apply the actual regex
4. Display highlighting uses the compiled regex directly (already supported by `FmtEntryHighlight`)
5. `-r` and keyword argument are mutually exclusive

## Testing

### `internal/filter/filter_test.go`

- `MatchSubstring`: case-insensitive match across each of the 5 fields
- `MatchSubstring`: no-match returns empty slice
- `MatchRegex`: regex matches across each of the 5 searchable fields individually
- `MatchRegex`: no-match returns empty slice
- `MatchRegex`: invalid regex returns error
- `MatchRegex`: empty pattern returns all entries

### `internal/db/db_test.go`

- `LoadFiltered` with single tool filter
- `LoadFiltered` with multiple comma-separated tools
- `LoadFiltered` with tag filter
- `LoadFiltered` with date, from, to, and from+to combinations
- `LoadFiltered` with combined tool + tag + date range
- Empty filters returns all entries (equivalent to `LoadAll`)
- No matches returns empty slice

### `cmd/export_test.go`

- `--date` and `--from`/`--to` mutual exclusivity produces error
- Invalid `-r` regex produces error
- Zero-match scenario prints message, no file created
- Comma-separated `--tool` and `--tag` parsing

### `cmd/show_test.go`

- Literal keyword search unchanged
- `-r` regex search works (e.g., `10\.0\.0\.\d+`)
- Invalid `-r` regex prints error message
- `-r` and keyword argument mutual exclusivity
