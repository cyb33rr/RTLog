# Compact Mode Truncation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Truncate long commands in `FmtCompact` so metadata (tag, note, output indicator) is always visible, right-aligned at the terminal edge.

**Architecture:** `FmtCompact` gains a `width` parameter. It builds the metadata suffix first, measures its visible width, then truncates the command to fit the remaining space and pads to right-align metadata. Note text is capped at 15 characters.

**Tech Stack:** Go, no new dependencies

**Spec:** `docs/superpowers/specs/2026-03-19-compact-mode-truncation-design.md`

---

### Task 1: Add `visibleLen` and `truncateText` helpers

**Files:**
- Modify: `internal/display/format.go` (append after line 184)
- Test: `internal/display/format_test.go`

- [ ] **Step 1: Write failing tests for `visibleLen`**

Add to `internal/display/format_test.go`:

```go
func TestVisibleLen(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"plain", "hello", 5},
		{"empty", "", 0},
		{"with_ansi", "\033[32mhello\033[0m", 5},
		{"nested_ansi", "\033[32mexit:0\033[0m  \033[2m8.1s\033[0m", 12},
		{"no_visible", "\033[0m", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := visibleLen(tt.in)
			if got != tt.want {
				t.Errorf("visibleLen(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/cyb3r/RTLog && go test ./internal/display/ -run TestVisibleLen -v`
Expected: FAIL — `visibleLen` undefined

- [ ] **Step 3: Implement `visibleLen`**

Add to `internal/display/format.go` after the `getFloat` function (after line 184):

```go
// visibleLen returns the number of visible runes in s, excluding ANSI escape sequences.
func visibleLen(s string) int {
	return len([]rune(RE_ANSI.ReplaceAllString(s, "")))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/cyb3r/RTLog && go test ./internal/display/ -run TestVisibleLen -v`
Expected: PASS

- [ ] **Step 5: Write failing tests for `truncateText`**

Add to `internal/display/format_test.go`:

```go
func TestTruncateText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"fits", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"truncated", "hello world", 5, "hell…"},
		{"one_char", "hello", 1, "…"},
		{"zero", "hello", 0, "…"},
		{"empty", "", 5, ""},
		{"unicode", "café latte", 6, "café …"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateText(tt.in, tt.max)
			if got != tt.want {
				t.Errorf("truncateText(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `cd /home/cyb3r/RTLog && go test ./internal/display/ -run TestTruncateText -v`
Expected: FAIL — `truncateText` undefined

- [ ] **Step 7: Implement `truncateText`**

Add to `internal/display/format.go` after `visibleLen`:

```go
// truncateText truncates plain text (no ANSI) to max visible runes, appending "…" if truncated.
func truncateText(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 0 {
		return "…"
	}
	return string(runes[:max-1]) + "…"
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `cd /home/cyb3r/RTLog && go test ./internal/display/ -run TestTruncateText -v`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/display/format.go internal/display/format_test.go
git commit -m "feat(display): add visibleLen and truncateText helpers"
```

---

### Task 2: Rewrite `FmtCompact` with width-aware layout

**Files:**
- Modify: `internal/display/format.go:103-142` (replace `FmtCompact`)
- Test: `internal/display/format_test.go`

- [ ] **Step 1: Write failing tests for the new `FmtCompact` signature**

Add to `internal/display/format_test.go`:

```go
func TestFmtCompactTruncatesLongCommand(t *testing.T) {
	entry := Entry{
		"ts":   "2025-01-15T14:22:01Z",
		"cmd":  "gobuster dir -u http://10.10.10.1/very/long/path/that/exceeds/the/terminal/width",
		"exit": 0,
		"dur":  8.0,
		"tag":  "recon",
		"note": "scan",
		"out":  "",
	}
	got := stripAnsi(FmtCompact(entry, 60))
	// Command should be truncated with …
	if !strings.Contains(got, "…") {
		t.Errorf("expected ellipsis in truncated command: %q", got)
	}
	// Metadata must be present
	if !strings.Contains(got, "exit:0") {
		t.Errorf("missing exit code: %q", got)
	}
	if !strings.Contains(got, "[recon]") {
		t.Errorf("missing tag: %q", got)
	}
	if !strings.Contains(got, "# scan") {
		t.Errorf("missing note: %q", got)
	}
	// Total visible width must not exceed 60
	if len([]rune(got)) > 60 {
		t.Errorf("visible width %d exceeds 60: %q", len([]rune(got)), got)
	}
}

func TestFmtCompactRightAlignsPadding(t *testing.T) {
	entry := Entry{
		"ts":   "2025-01-15T14:22:01Z",
		"cmd":  "ls",
		"exit": 0,
		"dur":  0.1,
		"tag":  "",
		"note": "",
		"out":  "",
	}
	got := stripAnsi(FmtCompact(entry, 60))
	// "ls" is short — line should be padded to 60 chars
	if len([]rune(got)) != 60 {
		t.Errorf("expected 60 visible chars, got %d: %q", len([]rune(got)), got)
	}
	// Metadata should be at the right edge — line ends with duration
	if !strings.HasSuffix(strings.TrimRight(got, " "), "0.1s") {
		t.Errorf("metadata not right-aligned: %q", got)
	}
}

func TestFmtCompactNoteTruncation(t *testing.T) {
	entry := Entry{
		"ts":   "2025-01-15T14:22:01Z",
		"cmd":  "nmap 10.0.0.1",
		"exit": 0,
		"dur":  1.0,
		"tag":  "",
		"note": "this is a very long note that should be truncated",
		"out":  "",
	}
	got := stripAnsi(FmtCompact(entry, 120))
	// Note should be capped at 15 chars with …
	if !strings.Contains(got, "# this is a very…") {
		// Find the note portion
		idx := strings.Index(got, "# ")
		if idx < 0 {
			t.Fatalf("note not found in output: %q", got)
		}
		noteEnd := got[idx+2:]
		// Extract just the note text (up to next field or end)
		parts := strings.SplitN(noteEnd, "  ", 2)
		noteText := parts[0]
		runes := []rune(noteText)
		if len(runes) > 15 {
			t.Errorf("note text exceeds 15 chars (%d): %q", len(runes), noteText)
		}
		if !strings.HasSuffix(noteText, "…") {
			t.Errorf("truncated note missing ellipsis: %q", noteText)
		}
	}
}

func TestFmtCompactEmptyTimestampPads(t *testing.T) {
	entry := Entry{
		"ts":   "",
		"cmd":  "nmap 10.0.0.1",
		"exit": 0,
		"dur":  1.0,
		"tag":  "",
		"note": "",
		"out":  "",
	}
	got := stripAnsi(FmtCompact(entry, 80))
	// Should start with 10 spaces (empty timestamp zone)
	if !strings.HasPrefix(got, "          ") {
		t.Errorf("expected 10-space prefix for empty timestamp, got: %q", got[:20])
	}
}

func TestFmtCompactNarrowTerminal(t *testing.T) {
	entry := Entry{
		"ts":   "2025-01-15T14:22:01Z",
		"cmd":  "nmap -sV -sC -p- 10.10.10.1",
		"exit": 0,
		"dur":  8.0,
		"tag":  "recon",
		"note": "scan",
		"out":  "output",
	}
	// Very narrow — command gets clamped to min 10 chars
	got := stripAnsi(FmtCompact(entry, 30))
	// Should not panic, should produce some output
	if len(got) == 0 {
		t.Errorf("expected non-empty output for narrow terminal")
	}
	// Command should still be partially visible
	if !strings.Contains(got, "nmap") {
		t.Errorf("command should be at least partially visible: %q", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cyb3r/RTLog && go test ./internal/display/ -run "TestFmtCompact(Truncates|RightAligns|NoteTruncation|EmptyTimestamp|NarrowTerminal)" -v`
Expected: FAIL — `FmtCompact` takes wrong number of arguments

- [ ] **Step 3: Rewrite `FmtCompact`**

Replace the `FmtCompact` function in `internal/display/format.go` (lines 103-142) with:

```go
// FmtCompact formats an entry as a compact single line for the Atuin-style TUI.
// Width is the terminal width; the output fits within it with metadata right-aligned.
// Format: <timestamp zone>  <command><padding><exit:N  Ns  [tag]  # note  [+out]>
func FmtCompact(entry Entry, width int) string {
	// Timestamp zone: always 10 visible chars (8-char time + 2-space gap, or 10 spaces)
	tsRaw, _ := entry["ts"].(string)
	ts := formatTimestamp(tsRaw)
	var tsZone string
	if ts == "" {
		tsZone = "          " // 10 spaces
	} else {
		tsZone = ts + "  "
	}

	// Command — collapse newlines (plain text, no ANSI)
	cmd := getString(entry, "cmd", "")
	cmd = strings.ReplaceAll(cmd, "\n", " ")

	// Build metadata suffix
	exitCode := getInt(entry, "exit", -1)
	var exitStr string
	if exitCode == 0 {
		exitStr = Colorize(fmt.Sprintf("exit:%d", exitCode), Green)
	} else {
		exitStr = Colorize(fmt.Sprintf("exit:%d", exitCode), Red)
	}

	dur := getFloat(entry, "dur", 0)
	durStr := Colorize(fmt.Sprintf("%gs", dur), Dim)

	tag := getString(entry, "tag", "")
	tagStr := ""
	if tag != "" {
		tagStr = "  " + Colorize(fmt.Sprintf("[%s]", tag), Yellow)
	}

	note := getString(entry, "note", "")
	noteStr := ""
	if note != "" {
		note = truncateText(note, 15)
		noteStr = "  # " + note
	}

	outIndicator := ""
	if out, _ := entry["out"].(string); out != "" {
		outIndicator = "  " + Colorize("[+out]", Dim)
	}

	meta := exitStr + "  " + durStr + tagStr + noteStr + outIndicator
	metaWidth := visibleLen(meta)

	// Command width budget: total - timestamp(10) - gutter(2) - metadata
	cmdWidth := width - 10 - 2 - metaWidth
	if cmdWidth < 10 {
		cmdWidth = 10
	}

	cmd = truncateText(cmd, cmdWidth)

	// Pad between command and metadata to right-align
	usedWidth := 10 + len([]rune(cmd)) + metaWidth
	padding := width - usedWidth
	if padding < 2 {
		padding = 2 // minimum gutter
	}

	return tsZone + cmd + strings.Repeat(" ", padding) + meta
}
```

- [ ] **Step 4: Update existing tests to pass width argument**

Update the 4 existing tests in `internal/display/format_test.go`:

In `TestFmtCompactBasic` (line 25): change `FmtCompact(entry)` to `FmtCompact(entry, 120)`

In `TestFmtCompactEmptyOptionalFields` (line 62): change `FmtCompact(entry)` to `FmtCompact(entry, 120)`

In `TestFmtCompactNewlinesCollapsed` (line 87): change `FmtCompact(entry)` to `FmtCompact(entry, 120)`

In `TestFmtCompactMissingTimestamp` (line 109): change `FmtCompact(entry)` to `FmtCompact(entry, 120)`

- [ ] **Step 5: Run all FmtCompact tests**

Run: `cd /home/cyb3r/RTLog && go test ./internal/display/ -run TestFmtCompact -v`
Expected: all 9 tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/display/format.go internal/display/format_test.go
git commit -m "feat(display): rewrite FmtCompact with width-aware truncation and right-aligned metadata"
```

---

### Task 3: Update Selector to pass terminal width to `FmtCompact`

**Files:**
- Modify: `internal/display/selector.go:340,375`

- [ ] **Step 1: Update `render()` to pass width**

In `internal/display/selector.go`, make two changes:

Line 340 — change:
```go
curLine := RE_ANSI.ReplaceAllString(FmtCompact(s.entries[entryIdx]), "")
```
to:
```go
curLine := RE_ANSI.ReplaceAllString(FmtCompact(s.entries[entryIdx], w), "")
```

Line 375 — change:
```go
line := FmtCompact(s.entries[entryIdx])
```
to:
```go
line := FmtCompact(s.entries[entryIdx], w)
```

- [ ] **Step 2: Run all tests**

Run: `cd /home/cyb3r/RTLog && go test ./internal/display/ -v`
Expected: all tests PASS

- [ ] **Step 3: Build and verify**

Run: `cd /home/cyb3r/RTLog && go build ./...`
Expected: no compilation errors

- [ ] **Step 4: Commit**

```bash
git add internal/display/selector.go
git commit -m "feat(display): pass terminal width to FmtCompact in Selector"
```

---

### Task 4: Final verification

- [ ] **Step 1: Run the full test suite**

Run: `cd /home/cyb3r/RTLog && go test ./... -v`
Expected: all tests PASS, no regressions

- [ ] **Step 2: Run go vet**

Run: `cd /home/cyb3r/RTLog && go vet ./...`
Expected: no issues
