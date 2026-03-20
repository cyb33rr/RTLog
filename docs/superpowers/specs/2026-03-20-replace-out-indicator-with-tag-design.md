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
- Note-mode display: tag slot replaces `[+out]` there too.

### `FmtEntry` (non-interactive format)

- Remove the `[+out]` indicator (lines 57-64).
- Remove the `showOutIndicator` variadic parameter from `FmtEntry` and `FmtEntryHighlight`.
- Tag display stays in its current inline position (alignment matters less in non-interactive output).

### No changes to

- `PrintOutputBlock` — output is still stored and viewable via `-a` flag / TUI expansion.
- Tag filtering in TUI (Tab key cycling).
- Database schema or tag storage.
- `show.go` callers — update call sites to drop the `showOutIndicator` argument.

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

Tags are right-aligned in a fixed 10-char column, maintaining consistent row alignment regardless of tag content.
