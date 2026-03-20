# Replace +out Indicator with Fixed-Width Tag Slot

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the `[+out]` indicator from all display formats and replace its slot in `FmtCompact` with a fixed 10-char tag field for consistent alignment.

**Architecture:** Two functions change: `FmtCompact` swaps its 7-char `[+out]` slot and variable-width inline tag for a single fixed 10-char tag slot at the end of metadata. `FmtEntry`/`FmtEntryHighlight` drop the `[+out]` indicator and `showOutIndicator` parameter entirely. Callers in `cmd/show.go` are updated to match the new signatures.

**Tech Stack:** Go, no new dependencies

**Spec:** `docs/superpowers/specs/2026-03-20-replace-out-indicator-with-tag-design.md`

---

### Task 1: Update FmtCompact tests for new tag slot behavior

**Files:**
- Modify: `internal/display/format_test.go:13-46` (TestFmtCompactBasic)
- Modify: `internal/display/format_test.go:48-71` (TestFmtCompactEmptyOptionalFields)
- Modify: `internal/display/format_test.go:141-160` (TestFmtCompactRightAlignsPadding)

- [ ] **Step 1: Update TestFmtCompactBasic**

This test has `note` set, so note-mode applies. The tag should now appear in the fixed slot even when a note is present. Remove the `[+out]` assertion (line 43-45), update the stale comment (line 33), and add a tag assertion since entries with both note+tag now show the tag:

```go
// When note exists, note replaces exit+dur; tag shows in fixed slot
if !strings.Contains(got, "# port scan") {
    t.Errorf("missing note in %q", got)
}
if strings.Contains(got, "exit:0") {
    t.Errorf("should not have exit code when note exists: %q", got)
}
if !strings.Contains(got, "[recon]") {
    t.Errorf("missing tag in fixed slot when note exists: %q", got)
}
if strings.Contains(got, "[+out]") {
    t.Errorf("should not have [+out] indicator: %q", got)
}
```

- [ ] **Step 2: Update TestFmtCompactEmptyOptionalFields**

This entry has empty tag and empty out. The bracket assertion on line 62-63 is still valid (no brackets when tag is empty). Just update the comment on line 33 if it references `[+out]`. The comment is actually on line 63 — update it:

```go
if strings.Contains(got, "[") {
    t.Errorf("should not have any brackets when tag is empty: %q", got)
}
```

- [ ] **Step 3: Update TestFmtCompactRightAlignsPadding**

This entry has no tag and no out. After the change, the line ends with the 10-space empty tag slot. Update the suffix assertion (line 157-159) to account for trailing spaces from the empty tag slot:

```go
// Metadata should be at the right edge — line ends with empty tag padding
trimmed := strings.TrimRight(got, " ")
if !strings.HasSuffix(trimmed, "0.1s") {
    t.Errorf("metadata not right-aligned: %q", got)
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `cd /home/cyb3r/RTLog && go test ./internal/display/ -run TestFmtCompact -v`
Expected: `TestFmtCompactBasic` FAILS (asserts `[+out]` present, asserts `[recon]` missing)

- [ ] **Step 5: Commit test updates**

```bash
git add internal/display/format_test.go
git commit -m "test(display): update FmtCompact tests for tag slot replacing +out"
```

---

### Task 2: Implement FmtCompact tag slot

**Files:**
- Modify: `internal/display/format.go:103-176` (FmtCompact function)

- [ ] **Step 1: Remove outIndicator and inline tag, add fixed tag slot**

Replace lines 123-157 of `FmtCompact` with:

```go
	// Build metadata suffix
	// exit(8) + "  " + dur(6) = 16 fixed chars before tag slot
	tag := getString(entry, "tag", "")
	var tagSlot string
	if tag != "" {
		// Truncate tag to fit in 8 chars (10 - 2 for brackets), pad to 10 visible
		tagText := truncateText(tag, 8)
		raw := fmt.Sprintf("[%s]", tagText)
		tagSlot = Colorize(raw, Yellow) + strings.Repeat(" ", 10-len([]rune(raw)))
	} else {
		tagSlot = "          " // 10 spaces
	}

	note := getString(entry, "note", "")
	var meta string
	if note != "" {
		// Note replaces exit+dur, left-aligned to exit column; tag slot stays
		noteText := "# " + truncateText(note, 15)
		meta = fmt.Sprintf("%-18s", noteText) + tagSlot
	} else {
		// No note: show full metadata
		exitCode := getInt(entry, "exit", -1)
		exitRaw := fmt.Sprintf("%-8s", fmt.Sprintf("exit:%d", exitCode))
		var exitStr string
		if exitCode == 0 {
			exitStr = Colorize(exitRaw, Green)
		} else {
			exitStr = Colorize(exitRaw, Red)
		}

		dur := getFloat(entry, "dur", 0)
		durStr := Colorize(fmt.Sprintf("%-6s", fmt.Sprintf("%gs", dur)), Dim)

		meta = exitStr + "  " + durStr + "  " + tagSlot
	}
```

- [ ] **Step 2: Update FmtCompact doc comment**

Change line 107 from:
```go
// Format: <timestamp zone>  <command><padding><exit:N  Ns  [tag]  # note  [+out]>
```
to:
```go
// Format: <timestamp zone>  <command><padding><exit:N  Ns  [tag(10)]  |  # note  [tag(10)]>
```

- [ ] **Step 3: Run tests to verify they pass**

Run: `cd /home/cyb3r/RTLog && go test ./internal/display/ -run TestFmtCompact -v`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add internal/display/format.go
git commit -m "feat(display): replace +out with fixed 10-char tag slot in FmtCompact"
```

---

### Task 3: Remove +out and showOutIndicator from FmtEntry

**Files:**
- Modify: `internal/display/format.go:15-84` (FmtEntry + FmtEntryHighlight)

- [ ] **Step 1: Update FmtEntry — remove outIndicator and showOutIndicator param**

Change the function signature on line 17 from:
```go
func FmtEntry(entry Entry, index, idxWidth int, showOutIndicator ...bool) string {
```
to:
```go
func FmtEntry(entry Entry, index, idxWidth int) string {
```

Remove lines 57-64 (the output indicator block).

Update the return on line 69 — remove `outIndicator` from the format string:
```go
return fmt.Sprintf("%s  %s  %s  %s  %s  %s  %s%s", idxStr, tsStr, toolStr, cmd, exitStr, durStr, tagStr, noteStr)
```

Update line 16 doc comment from:
```go
// Format: idx  HH:MM:SS  tool  cmd  exit:N  Ns  [tag]  # note  [+out]
```
to:
```go
// Format: idx  HH:MM:SS  tool  cmd  exit:N  Ns  [tag]  # note
```

- [ ] **Step 2: Update FmtEntryHighlight — remove showOutIndicator param**

Change line 73 from:
```go
func FmtEntryHighlight(entry Entry, pattern *regexp.Regexp, index, idxWidth int, showOutIndicator ...bool) string {
```
to:
```go
func FmtEntryHighlight(entry Entry, pattern *regexp.Regexp, index, idxWidth int) string {
```

Change line 74 from:
```go
line := FmtEntry(entry, index, idxWidth, showOutIndicator...)
```
to:
```go
line := FmtEntry(entry, index, idxWidth)
```

- [ ] **Step 3: Verify compilation**

Run: `cd /home/cyb3r/RTLog && go build ./internal/display/`
Expected: PASS (this package compiles on its own)

- [ ] **Step 4: Commit**

```bash
git add internal/display/format.go
git commit -m "refactor(display): remove +out indicator and showOutIndicator param from FmtEntry"
```

---

### Task 4: Update callers in cmd/show.go

**Files:**
- Modify: `cmd/show.go:99,165,228`

- [ ] **Step 1: Drop the `false` argument from three call sites**

Line 99 — change:
```go
fmt.Println(display.FmtEntryHighlight(m, pattern, i+1, idxWidth, false))
```
to:
```go
fmt.Println(display.FmtEntryHighlight(m, pattern, i+1, idxWidth))
```

Line 165 — change:
```go
fmt.Println(display.FmtEntryHighlight(m, re, i+1, idxWidth, false))
```
to:
```go
fmt.Println(display.FmtEntryHighlight(m, re, i+1, idxWidth))
```

Line 228 — change:
```go
fmt.Println(display.FmtEntry(m, origIdx(i), idxWidth, false))
```
to:
```go
fmt.Println(display.FmtEntry(m, origIdx(i), idxWidth))
```

- [ ] **Step 2: Verify full build**

Run: `cd /home/cyb3r/RTLog && go build ./...`
Expected: PASS

- [ ] **Step 3: Run all tests**

Run: `cd /home/cyb3r/RTLog && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/show.go
git commit -m "refactor(cmd): update show.go callers for removed showOutIndicator param"
```
