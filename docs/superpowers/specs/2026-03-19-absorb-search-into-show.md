# Absorb `search` into `show`

**Date:** 2026-03-19
**Status:** Approved

## Problem

The `search` and `show` commands have overlapping functionality. The `show` TUI already supports live text filtering across 5 fields. Maintaining a separate `search` command adds CLI surface area without enough differentiation.

## Decision

Remove the `search` command. Absorb its functionality into `show` via an optional positional keyword argument.

## CLI Interface

```
rtlog show [keyword]                     # non-interactive search results
rtlog show --today [keyword]             # search within today's entries
rtlog show --date YYYY-MM-DD [keyword]   # search within a specific date
rtlog show                               # TUI (unchanged)
rtlog show -a [keyword]                  # search with full captured output
```

When a keyword is present, output is always non-interactive — immediate results printed to stdout. When no keyword is present, `show` behaves exactly as it does today.

## Query Logic

- **Keyword, no date flags:** `db.Search(keyword)` — searches all entries.
- **Keyword + `--date`/`--today`:** New `db.SearchByDate(keyword, dateStr)` — combines date prefix filtering with keyword search.
- **No keyword:** Existing `LoadAll()` / `LoadByDate()` behavior (unchanged).

### Search Scope

Search narrows from 7 fields to 5, dropping `user` and `host`:

| Field | Searched |
|-------|----------|
| cmd   | yes      |
| tool  | yes      |
| cwd   | yes      |
| tag   | yes      |
| note  | yes      |
| user  | no       |
| host  | no       |

This aligns the DB search with the TUI's live filter scope.

## Output Format (keyword present)

1. Bold header: `--- N match(es) for 'keyword' in engagement ---` (with `[YYYY-MM-DD]` suffix if date-filtered)
2. Entries in chronological order (oldest first)
3. Each entry formatted with `FmtEntry` + keyword highlighting via `FmtEntryHighlight`
4. If `-a` flag is set, captured output blocks are printed below each matching entry
5. Dim footer: `N result(s)`

## Changes

### Modified Files

- **`cmd/show.go`**: Accept optional positional arg (`cobra.MaximumNArgs(1)`). When keyword is present, use `db.Search` or `db.SearchByDate`, format with `FmtEntryHighlight`, print non-interactively. Update `Use` to `"show [keyword]"`.
- **`internal/db/db.go`**: Update `Search` to query 5 fields instead of 7. Add `SearchByDate(keyword, dateStr)` method combining date prefix + keyword filtering.

### Deleted Files

- **`cmd/search.go`**: Removed entirely.

### Unchanged

- `internal/display/format.go`: `FmtEntryHighlight` stays as-is (now used by `show`).
- `db.Search` method stays (signature unchanged, just fewer fields in the WHERE clause).
- TUI behavior when no keyword is provided — completely unchanged.
