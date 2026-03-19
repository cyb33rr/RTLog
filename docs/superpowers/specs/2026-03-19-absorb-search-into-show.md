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

**Note:** Adding `cobra.MaximumNArgs(1)` means `rtlog show foo bar` (multiple args) will now error. Previously Cobra silently ignored extra args. This is intentional — extra positional args were never meaningful.

## Query Logic

- **Keyword, no date flags:** `db.Search(keyword)` — searches all entries.
- **Keyword + `--date`/`--today`:** New `db.SearchByDate(keyword, dateStr)` — single SQL query: `WHERE ts LIKE 'dateStr%' AND (cmd LIKE '%kw%' OR tool LIKE '%kw%' OR cwd LIKE '%kw%' OR tag LIKE '%kw%' OR note LIKE '%kw%')`.
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

1. Bold header: `--- N match(es) for 'keyword' in <resolved engagement name> ---` (with `[YYYY-MM-DD]` suffix if date-filtered)
2. Entries in chronological order (oldest first) — this intentionally differs from `show -a` / pipe mode which use newest-first, because search results read more naturally as a chronological narrative
3. Each entry formatted with `FmtEntry` + keyword highlighting via `FmtEntryHighlight`
4. If `-a` flag is also set, captured output blocks are printed below each matching entry using `PrintOutputBlock(m, true)` with the `[+out]` indicator suppressed (matching existing `show -a` behavior: `FmtEntry(m, idx, idxWidth, false)`)
5. Dim footer: `N result(s)`

## Changes

### Modified Files

- **`cmd/show.go`**: Accept optional positional arg (`cobra.MaximumNArgs(1)`). When keyword is present, use `db.Search` or `db.SearchByDate`, format with `FmtEntryHighlight`, print non-interactively. Update `Use` to `"show [keyword]"`.
- **`internal/db/db.go`**: Update `Search` to query 5 fields instead of 7. Add `SearchByDate(keyword, dateStr)` method combining date prefix + keyword filtering.
- **`internal/db/db_test.go`**: Update `TestSearchUser` (which tests that searching "alice" matches by `user` field) — this test must be updated or removed since `user` is no longer searched.
- **`README.md`**: Update usage examples to remove `rtlog search <keyword>` and document the new `rtlog show [keyword]` syntax.
- **`internal/display/format.go`**: Add a `showOutIndicator` variadic bool parameter to `FmtEntryHighlight` (matching `FmtEntry`'s existing pattern) so callers can suppress the `[+out]` indicator when `-a` is active. The parameter is passed through to the inner `FmtEntry` call.

### Deleted Files

- **`cmd/search.go`**: Removed entirely.

### Unchanged

- `db.Search` method stays (signature unchanged, just fewer fields in the WHERE clause and updated doc comment).
- TUI behavior when no keyword is provided — completely unchanged.
