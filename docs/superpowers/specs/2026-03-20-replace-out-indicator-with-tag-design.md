# Replace +out Indicator with Fixed-Width Tag Slot

## Problem

The `[+out]` indicator in `rtlog show` occupies a fixed 7-char slot at the end of the compact format metadata. Meanwhile, the tag field is variable-width and inserted inline before `[+out]`, breaking alignment across rows when tags have different lengths.

## Solution

Remove the `[+out]` indicator entirely and replace its slot with a fixed 10-char tag field. This solves alignment by giving tags a consistent, padded position.

## Changes

### `FmtCompact` (compact TUI format)

- Remove the 7-char `[+out]` indicator slot (lines 125-128).
- Remove the variable-width tag from inline metadata (lines 150-154, line 156).
- Add a fixed 10-char tag slot at the end of metadata where `[+out]` was:
  - Tag present: yellow `[tagname]` truncated and right-padded to 10 visible chars.
  - Tag empty: 10 spaces.
- Command width budget shrinks by 3 chars (10 - 7) to accommodate the wider slot.
- Note-mode display: tag slot replaces `[+out]` in the note path (line 135). When an entry has both a note and a tag, the tag is still visible in the fixed slot.
- Update format comment on line 107 to remove `[+out]` from the format description.

### `FmtEntry` (non-interactive format)

- Remove the `[+out]` indicator (lines 57-64).
- Remove the `showOutIndicator` variadic parameter from `FmtEntry` and `FmtEntryHighlight`.
- Tag display stays in its current inline position (alignment matters less in non-interactive output).
- Update format comment on line 16 to remove `[+out]` from the format description.

### Caller updates

Update call sites that pass `showOutIndicator` — these will fail to compile after the parameter is removed:

- `cmd/show.go` line 99: `FmtEntryHighlight(m, pattern, i+1, idxWidth, false)` — drop `false`
- `cmd/show.go` line 165: `FmtEntryHighlight(m, re, i+1, idxWidth, false)` — drop `false`
- `cmd/show.go` line 228: `FmtEntry(m, origIdx(i), idxWidth, false)` — drop `false`

Callers that omit the parameter (`cmd/show.go` lines 102, 168, 259; `cmd/delete.go` line 49; `cmd/timeline.go` line 113) need no code change.

### Test updates

- `format_test.go` `TestFmtCompactBasic` (lines 43-45): Remove assertion that `[+out]` is present; add assertion for fixed-width tag slot instead.
- `format_test.go` `TestFmtCompactEmptyOptionalFields` (line 33): Update stale comment referencing `[+out]`.
- Update any test entries or assertions that reference the old format.

### No changes to

- `PrintOutputBlock` — output is still stored and viewable via `-a` flag / TUI expansion.
- Tag filtering in TUI (Tab key cycling).
- Database schema or tag storage.

## Format Examples

Before (compact):
```
12:30:45  nmap -sV 10.10.10.1    exit:0    5s  [recon]  [+out]
12:31:00  gobuster dir -u ...     exit:0    3s           [+out]
```

After (compact):
```
12:30:45  nmap -sV 10.10.10.1    exit:0    5s  [recon]
12:31:00  gobuster dir -u ...     exit:0    3s
```

The tag occupies a fixed 10-char slot at the end. Entries with no tag get 10 spaces, maintaining consistent alignment.
