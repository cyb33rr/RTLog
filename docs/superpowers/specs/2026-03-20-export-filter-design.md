# Export Filtering & Regex Search

## Problem

`rtlog export` exports all entries unconditionally. Users need to export subsets filtered by tool, tag, date range, and free-text regex. Additionally, `rtlog show`'s keyword search uses literal substring matching (LIKE + `QuoteMeta`), limiting search power.

## Solution

Add stackable filter flags to `export` and upgrade `show`'s search to regex.

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
| `--filter` | string | Regex matched against cmd, tool, cwd, tag, note  |

Rules:
- `--date` is mutually exclusive with `--from`/`--to`
- `--from` and `--to` can each be used alone (open-ended range)
- All flags combine with AND logic across different flag types
- `--tool` and `--tag` use OR logic within their comma-separated values
- Invalid `--filter` regex exits with a clear error message

Example:
```
rtlog export csv --tool nmap,nxc --tag recon --from 2026-03-15 --to 2026-03-17 --filter "10\.0\.0\.\d+"
```

### Show

The keyword argument (`rtlog show <keyword>`) changes from literal substring matching to regex matching. Invalid regex prints an error and exits.

## Architecture

**Hybrid approach:** SQL-level filtering for structured fields (tool, tag, date/epoch) leveraging existing indices. Go-level filtering for the `--filter` regex using native `regexp`.

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

### Regex Filter (`internal/filter/filter.go`)

New package with a single function:

```go
func MatchRegex(entries []logfile.LogEntry, pattern string) ([]logfile.LogEntry, error)
```

- Compiles `pattern` with `regexp.Compile`
- Returns error if pattern is invalid
- Tests each entry against 5 fields: `cmd`, `tool`, `cwd`, `tag`, `note`
- Entry matches if any field matches the regex
- Returns filtered slice (empty slice, not nil, when no matches)

### Export Command (`cmd/export.go`)

Updated flow:

1. Parse and validate flags
   - Mutual exclusivity: `--date` vs `--from`/`--to`
   - Validate `--filter` regex at parse time
   - Split `--tool` and `--tag` on commas
   - Validate date formats for `--date`, `--from`, `--to`
2. Call `db.LoadFiltered(tools, tags, date, from, to)` instead of `db.LoadAll()`
3. If `--filter` is set, pass results through `filter.MatchRegex()`
4. If zero entries remain: print `"No entries match the given filters"` to stderr and exit (no file created)
5. Otherwise pass to format-specific exporter as before

### Show Command (`cmd/show.go`)

Updated search behavior:

1. The keyword argument is compiled as a regex pattern (instead of being wrapped with `QuoteMeta`)
2. Invalid regex prints a clear error and exits
3. The DB-level `Search()`/`SearchByDate()` still does the initial LIKE query for broad matching
4. Results are refined with `filter.MatchRegex()` to apply the actual regex
5. Display highlighting uses the compiled regex directly (already supported by `FmtEntryHighlight`)

## Testing

### `internal/filter/filter_test.go`

- Regex matches across each of the 5 searchable fields individually
- No-match returns empty slice
- Invalid regex returns error
- Empty pattern returns all entries

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
- Invalid `--filter` regex produces error
- Zero-match scenario prints message, no file created
- Comma-separated `--tool` and `--tag` parsing

### `cmd/show_test.go`

- Regex keyword search works (e.g., `10\.0\.0\.\d+`)
- Invalid regex prints error message
