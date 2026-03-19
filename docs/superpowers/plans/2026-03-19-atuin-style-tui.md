# Atuin-Style TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace RTLog's menu-based selector with an Atuin-style TUI: compact colored rows (newest at bottom), inline text filtering, and preset toggle filters (tag cycling, failed-only).

**Architecture:** Evolve the existing `Selector` struct in `internal/display/selector.go` by adding filter state, a `filtered []int` index slice, and rewriting the render/input loop. Add a new `FmtCompact()` formatter in `format.go`. Minimal changes to `cmd/show.go` (remove manual reversal and header). No new dependencies.

**Tech Stack:** Go, `golang.org/x/term` (raw mode), ANSI escape codes

**Spec:** `docs/superpowers/specs/2026-03-19-atuin-style-tui-design.md`

---

### Task 1: Add `FmtCompact()` formatter

**Files:**
- Modify: `internal/display/format.go`
- Create: `internal/display/format_test.go`

This is a pure function with no side effects — straightforward to TDD.

Format: `HH:MM:SS  cmd  exit:N  Ns  [tag]  # note  [+out]`

No index number, no tool name. Reuses existing helpers.

- [ ] **Step 1: Write failing tests for `FmtCompact()`**

Create `internal/display/format_test.go`:

```go
package display

import (
	"strings"
	"testing"
)

// stripAnsi removes ANSI escape codes for test assertions.
func stripAnsi(s string) string {
	return RE_ANSI.ReplaceAllString(s, "")
}

func TestFmtCompactBasic(t *testing.T) {
	entry := Entry{
		"ts":   "2025-01-15T14:22:01Z",
		"cmd":  "nmap -sV 10.0.0.1",
		"exit": 0,
		"dur":  8.1,
		"tag":  "recon",
		"note": "port scan",
		"out":  "some output",
		"tool": "nmap",
		"cwd":  "/tmp",
	}
	got := stripAnsi(FmtCompact(entry))

	// Should contain timestamp
	if !strings.Contains(got, "14:22:01") {
		t.Errorf("missing timestamp in %q", got)
	}
	// Should contain command
	if !strings.Contains(got, "nmap -sV 10.0.0.1") {
		t.Errorf("missing command in %q", got)
	}
	// Should contain exit code
	if !strings.Contains(got, "exit:0") {
		t.Errorf("missing exit code in %q", got)
	}
	// Should contain duration
	if !strings.Contains(got, "8.1s") {
		t.Errorf("missing duration in %q", got)
	}
	// Should contain tag
	if !strings.Contains(got, "[recon]") {
		t.Errorf("missing tag in %q", got)
	}
	// Should contain note
	if !strings.Contains(got, "# port scan") {
		t.Errorf("missing note in %q", got)
	}
	// Should contain output indicator
	if !strings.Contains(got, "[+out]") {
		t.Errorf("missing [+out] in %q", got)
	}
}

func TestFmtCompactEmptyOptionalFields(t *testing.T) {
	entry := Entry{
		"ts":   "2025-01-15T10:00:00Z",
		"cmd":  "gobuster dir -u http://target",
		"exit": 1,
		"dur":  3.0,
		"tag":  "",
		"note": "",
		"out":  "",
		"tool": "gobuster",
		"cwd":  "/tmp",
	}
	got := stripAnsi(FmtCompact(entry))

	// No tag, no note, no output — should have no brackets, no #, no [+out]
	if strings.Contains(got, "[") {
		t.Errorf("should not have any brackets when tag and out are empty: %q", got)
	}
	if strings.Contains(got, "#") {
		t.Errorf("should not have note marker when note is empty: %q", got)
	}
}

func TestFmtCompactNewlinesCollapsed(t *testing.T) {
	entry := Entry{
		"ts":   "2025-01-15T10:00:00Z",
		"cmd":  "echo hello\necho world",
		"exit": 0,
		"dur":  0.1,
		"tag":  "",
		"note": "",
		"out":  "",
		"tool": "echo",
		"cwd":  "/tmp",
	}
	got := stripAnsi(FmtCompact(entry))

	if strings.Contains(got, "\n") {
		t.Errorf("command newlines should be collapsed: %q", got)
	}
	if !strings.Contains(got, "echo hello echo world") {
		t.Errorf("collapsed command not found in %q", got)
	}
}

func TestFmtCompactMissingTimestamp(t *testing.T) {
	entry := Entry{
		"ts":   "",
		"cmd":  "nmap -sV 10.0.0.1",
		"exit": 0,
		"dur":  1.0,
		"tag":  "",
		"note": "",
		"out":  "",
		"tool": "nmap",
		"cwd":  "/tmp",
	}
	// Should not panic with empty/missing timestamp
	got := stripAnsi(FmtCompact(entry))
	if !strings.Contains(got, "nmap") {
		t.Errorf("command missing from output: %q", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cyb3r/RTLog && go test ./internal/display/ -run TestFmtCompact -v`
Expected: compilation error — `FmtCompact` is not defined.

- [ ] **Step 3: Implement `FmtCompact()`**

Add to `internal/display/format.go`, after the existing `FmtEntry` function:

```go
// FmtCompact formats an entry as a compact single line for the Atuin-style TUI.
// Format: HH:MM:SS  cmd  exit:N  Ns  [tag]  # note  [+out]
// No index number, no separate tool name.
func FmtCompact(entry Entry) string {
	tsRaw, _ := entry["ts"].(string)
	tsStr := formatTimestamp(tsRaw)

	cmd := getString(entry, "cmd", "")
	cmd = strings.ReplaceAll(cmd, "\n", " ")

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
		noteStr = "  # " + note
	}

	outIndicator := ""
	if out, _ := entry["out"].(string); out != "" {
		outIndicator = "  " + Colorize("[+out]", Dim)
	}

	return fmt.Sprintf("%s  %s  %s  %s%s%s%s", tsStr, cmd, exitStr, durStr, tagStr, noteStr, outIndicator)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cyb3r/RTLog && go test ./internal/display/ -run TestFmtCompact -v`
Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/display/format.go internal/display/format_test.go
git commit -m "feat(display): add FmtCompact formatter for Atuin-style TUI"
```

---

### Task 2: Add filtering logic (`ApplyFilters` and `CollectTags`)

**Files:**
- Modify: `internal/display/selector.go`
- Create: `internal/display/selector_test.go`

These are pure functions operating on `[]Entry` — easy to test in isolation without raw terminal mode. We add them as exported package-level functions. The existing `Selector` struct and methods are untouched in this task.

- [ ] **Step 1: Write failing tests for filtering**

Create `internal/display/selector_test.go`:

```go
package display

import (
	"testing"
)

func makeEntries() []Entry {
	return []Entry{
		{"cmd": "nmap -sV 10.0.0.1", "tool": "nmap", "tag": "recon", "note": "scan", "cwd": "/tmp", "exit": 0, "out": ""},
		{"cmd": "gobuster dir -u http://target", "tool": "gobuster", "tag": "recon", "note": "", "cwd": "/opt", "exit": 1, "out": ""},
		{"cmd": "evil-winrm -i 10.0.0.1", "tool": "evil-winrm", "tag": "exploitation", "note": "got shell", "cwd": "/tmp", "exit": 0, "out": "output"},
		{"cmd": "nmap -p- 10.0.0.2", "tool": "nmap", "tag": "recon", "note": "", "cwd": "/tmp", "exit": 0, "out": ""},
		{"cmd": "crackmapexec smb 10.0.0.0/24", "tool": "crackmapexec", "tag": "", "note": "", "cwd": "/tmp", "exit": 2, "out": ""},
	}
}

func TestApplyFiltersNoFilter(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, "", "", false)
	if len(filtered) != 5 {
		t.Errorf("got %d, want 5 (no filter = all entries)", len(filtered))
	}
}

func TestApplyFiltersTextFilter(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, "nmap", "", false)
	if len(filtered) != 2 {
		t.Errorf("got %d, want 2 (two nmap entries)", len(filtered))
	}
	if filtered[0] != 0 || filtered[1] != 3 {
		t.Errorf("got indices %v, want [0 3]", filtered)
	}
}

func TestApplyFiltersTextFilterCaseInsensitive(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, "NMAP", "", false)
	if len(filtered) != 2 {
		t.Errorf("got %d, want 2 (case-insensitive)", len(filtered))
	}
}

func TestApplyFiltersTextFilterMatchesCwd(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, "/opt", "", false)
	if len(filtered) != 1 {
		t.Errorf("got %d, want 1 (one entry with cwd /opt)", len(filtered))
	}
	if filtered[0] != 1 {
		t.Errorf("got index %d, want 1", filtered[0])
	}
}

func TestApplyFiltersTextFilterMatchesNote(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, "shell", "", false)
	if len(filtered) != 1 {
		t.Errorf("got %d, want 1 (one entry with 'shell' in note)", len(filtered))
	}
}

func TestApplyFiltersTagFilter(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, "", "recon", false)
	if len(filtered) != 3 {
		t.Errorf("got %d, want 3 (three recon entries)", len(filtered))
	}
}

func TestApplyFiltersFailOnly(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, "", "", true)
	if len(filtered) != 2 {
		t.Errorf("got %d, want 2 (two non-zero exit entries)", len(filtered))
	}
}

func TestApplyFiltersStacked(t *testing.T) {
	entries := makeEntries()
	// Text "nmap" + tag "recon" + fail only → should be 0 (nmap recon entries all exit 0)
	filtered := ApplyFilters(entries, "nmap", "recon", true)
	if len(filtered) != 0 {
		t.Errorf("got %d, want 0 (nmap+recon+fail = none)", len(filtered))
	}
}

func TestApplyFiltersNoMatches(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, "zzzznotfound", "", false)
	if len(filtered) != 0 {
		t.Errorf("got %d, want 0", len(filtered))
	}
}

func TestCollectTags(t *testing.T) {
	entries := makeEntries()
	tags := CollectTags(entries)

	if len(tags) != 2 {
		t.Fatalf("got %d tags, want 2: %v", len(tags), tags)
	}
	// CollectTags returns sorted tags
	if tags[0] != "exploitation" || tags[1] != "recon" {
		t.Errorf("got tags %v, want [exploitation recon]", tags)
	}
}

func TestCollectTagsNoTags(t *testing.T) {
	entries := []Entry{
		{"cmd": "ls", "tool": "ls", "tag": "", "exit": 0},
	}
	tags := CollectTags(entries)
	if len(tags) != 0 {
		t.Errorf("got %d tags, want 0 for entries with no tags", len(tags))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cyb3r/RTLog && go test ./internal/display/ -run "TestApplyFilters|TestCollectTags" -v`
Expected: compilation error — `ApplyFilters` and `CollectTags` are not defined.

- [ ] **Step 3: Implement `ApplyFilters` and `CollectTags`**

Add to `internal/display/selector.go`, before the `Selector` struct definition. Also add `"sort"` to the existing import block in `selector.go`.

```go
// ApplyFilters returns indices into entries that match all active filters.
// textFilter is case-insensitive substring across cmd, tool, tag, note, cwd.
// tagFilter is exact match on tag ("" = no filter).
// failOnly filters to non-zero exit codes.
func ApplyFilters(entries []Entry, textFilter, tagFilter string, failOnly bool) []int {
	var result []int
	lower := strings.ToLower(textFilter)
	for i, e := range entries {
		if tagFilter != "" {
			if getString(e, "tag", "") != tagFilter {
				continue
			}
		}
		if failOnly {
			if getInt(e, "exit", 0) == 0 {
				continue
			}
		}
		if lower != "" {
			fields := []string{
				strings.ToLower(getString(e, "cmd", "")),
				strings.ToLower(getString(e, "tool", "")),
				strings.ToLower(getString(e, "tag", "")),
				strings.ToLower(getString(e, "note", "")),
				strings.ToLower(getString(e, "cwd", "")),
			}
			found := false
			for _, f := range fields {
				if strings.Contains(f, lower) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		result = append(result, i)
	}
	if result == nil {
		result = []int{}
	}
	return result
}

// CollectTags returns unique non-empty tags from all entries, sorted.
func CollectTags(entries []Entry) []string {
	seen := map[string]bool{}
	for _, e := range entries {
		tag := getString(e, "tag", "")
		if tag != "" {
			seen[tag] = true
		}
	}
	tags := make([]string, 0, len(seen))
	for tag := range seen {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}
```

Note: `getString` and `getInt` are defined in `format.go` in the same package — accessible without import.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cyb3r/RTLog && go test ./internal/display/ -run "TestApplyFilters|TestCollectTags" -v`
Expected: all 11 tests PASS.

- [ ] **Step 5: Run all existing tests to check for regressions**

Run: `cd /home/cyb3r/RTLog && go test ./...`
Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/display/selector.go internal/display/selector_test.go
git commit -m "feat(display): add ApplyFilters and CollectTags for TUI filtering"
```

---

### Task 3: Rewrite Selector (struct, input, render) as atomic change

**Files:**
- Modify: `internal/display/selector.go`

This task replaces the entire `Selector` — struct, constructor, `Run()`, `render()`, and navigation methods — in one atomic step. This avoids intermediate non-compilable states. The `clearPrev()`, `truncateVisible()`, `scrollOutUp()`, and `scrollOutDown()` methods are unchanged.

- [ ] **Step 1: Replace the `Selector` struct and `NewSelector`**

Replace the existing `Selector` struct and `NewSelector` function (currently at lines 12-32 of `selector.go`) with:

```go
// Selector provides an interactive terminal UI for browsing log entries.
type Selector struct {
	entries   []Entry // all entries, chronological (oldest first)
	cursor    int     // position in filtered slice
	offset    int     // scroll offset in filtered slice
	expanded  bool
	outScroll int
	lastLines int

	// Filtering
	filter    string // text filter input
	tagFilter string // "" = all
	failOnly  bool
	filtered  []int    // indices into entries matching current filters
	allTags   []string // unique tags for Tab cycling
	tagIdx    int      // current position in tag cycle (0 = "all")
}

// NewSelector creates a Selector for the given entries (chronological order, oldest first).
func NewSelector(entries []Entry) *Selector {
	s := &Selector{
		entries: entries,
	}
	s.allTags = CollectTags(entries)
	s.filtered = ApplyFilters(entries, "", "", false)
	// Cursor starts at bottom (newest = last in filtered)
	if len(s.filtered) > 0 {
		s.cursor = len(s.filtered) - 1
	}
	return s
}
```

- [ ] **Step 2: Add the `applyAndReset` and `renderFilterBar` helper methods**

Add after the `scrollOutUp` method:

```go
// applyAndReset rebuilds the filtered slice and resets cursor to newest.
func (s *Selector) applyAndReset() {
	s.filtered = ApplyFilters(s.entries, s.filter, s.tagFilter, s.failOnly)
	if len(s.filtered) > 0 {
		s.cursor = len(s.filtered) - 1
	} else {
		s.cursor = 0
	}
	s.offset = 0
	s.expanded = false
	s.outScroll = 0
}

// renderFilterBar builds the filter bar string.
func (s *Selector) renderFilterBar() string {
	var parts []string

	if s.tagFilter != "" {
		parts = append(parts, Colorize(fmt.Sprintf("[%s]", s.tagFilter), Yellow))
	}
	if s.failOnly {
		parts = append(parts, Colorize("[!fail]", Red))
	}

	total := len(s.entries)
	matched := len(s.filtered)
	hasFilter := s.filter != "" || s.tagFilter != "" || s.failOnly
	if hasFilter {
		parts = append(parts, fmt.Sprintf("%d/%d matches", matched, total))
	} else {
		parts = append(parts, fmt.Sprintf("%d entries", total))
	}

	parts = append(parts, fmt.Sprintf("▸ %s_", s.filter))

	return "  " + strings.Join(parts, "   ")
}
```

- [ ] **Step 3: Replace the `Run()` method**

Replace the existing `Run()` method with:

```go
// Run enters raw mode and runs the interactive selector loop.
func (s *Selector) Run() error {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("failed to enter raw mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	os.Stdout.WriteString("\033[?25l")
	defer func() {
		s.clearPrev()
		os.Stdout.WriteString("\033[?25h")
	}()

	buf := make([]byte, 3)
	for {
		s.clearPrev()
		s.render()

		n, err := os.Stdin.Read(buf)
		if err != nil {
			return nil
		}

		// Escape sequence (arrow keys, etc.)
		if n == 3 && buf[0] == 27 && buf[1] == '[' {
			switch buf[2] {
			case 'A': // Up arrow
				if s.expanded {
					s.scrollOutUp()
				} else {
					s.moveUp()
				}
			case 'B': // Down arrow
				if s.expanded {
					s.scrollOutDown()
				} else {
					s.moveDown()
				}
			}
			continue
		}

		if n == 1 {
			switch buf[0] {
			case 27: // Esc (lone byte = quit)
				return nil
			case 13: // Enter
				if len(s.filtered) > 0 {
					s.expanded = !s.expanded
					s.outScroll = 0
				}
			case 9: // Tab — cycle tag filter
				if len(s.allTags) > 0 {
					s.tagIdx = (s.tagIdx + 1) % (len(s.allTags) + 1)
					if s.tagIdx == 0 {
						s.tagFilter = ""
					} else {
						s.tagFilter = s.allTags[s.tagIdx-1]
					}
					s.applyAndReset()
				}
			case 6: // Ctrl+F — toggle failed only
				s.failOnly = !s.failOnly
				s.applyAndReset()
			case 127, 8: // Backspace (DEL or BS)
				if len(s.filter) > 0 {
					runes := []rune(s.filter)
					s.filter = string(runes[:len(runes)-1])
					s.applyAndReset()
				}
			default:
				// Printable ASCII
				if buf[0] >= 0x20 && buf[0] <= 0x7E {
					s.filter += string(buf[0])
					s.applyAndReset()
				}
			}
		}
	}
}
```

- [ ] **Step 4: Replace `moveUp`, `moveDown`, and delete `origIdx`**

Replace the existing `moveDown` and `moveUp` methods with:

```go
func (s *Selector) moveDown() {
	if s.cursor < len(s.filtered)-1 {
		s.cursor++
		s.outScroll = 0
	}
}

func (s *Selector) moveUp() {
	if s.cursor > 0 {
		s.cursor--
		s.outScroll = 0
	}
}
```

Delete the `origIdx` method entirely — no longer needed.

- [ ] **Step 5: Replace `render()`**

Replace the existing `render()` method with:

```go
func (s *Selector) render() {
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	if h < 5 {
		h = 20
	}
	if w <= 0 {
		w = 80
	}

	var b strings.Builder
	lines := 0

	writeln := func(text string) {
		b.WriteString(truncateVisible(text, w))
		b.WriteString("\r\n")
		lines++
	}

	// Calculate space for entries
	var outLines []string
	var wrapExtra int
	if s.expanded && len(s.filtered) > 0 {
		outLines = s.getOutputLines(h)
		// Account for wrap lines of the selected entry
		entryIdx := s.filtered[s.cursor]
		curLine := RE_ANSI.ReplaceAllString(FmtCompact(s.entries[entryIdx]), "")
		nRunes := len([]rune(curLine))
		if nRunes > w {
			wrapExtra = (nRunes - 1) / w
		}
	}

	// Layout: entries + blank line + filter bar (no trailing \r\n on filter bar)
	entrySlots := h - 2 - len(outLines) - wrapExtra
	if entrySlots < 1 {
		entrySlots = 1
	}

	if len(s.filtered) == 0 {
		msg := "(no matches)"
		if len(s.entries) == 0 {
			msg = "(no entries)"
		}
		writeln("")
		writeln(Colorize("    "+msg, Dim))
		writeln("")
	} else {
		// Scrolling: keep cursor visible
		if s.cursor < s.offset {
			s.offset = s.cursor
		}
		if s.cursor >= s.offset+entrySlots {
			s.offset = s.cursor - entrySlots + 1
		}

		end := s.offset + entrySlots
		if end > len(s.filtered) {
			end = len(s.filtered)
		}

		for i := s.offset; i < end; i++ {
			entryIdx := s.filtered[i]
			line := FmtCompact(s.entries[entryIdx])
			if i == s.cursor {
				plain := RE_ANSI.ReplaceAllString(line, "")
				if s.expanded {
					runes := []rune(plain)
					for off := 0; off < len(runes); off += w {
						wEnd := off + w
						if wEnd > len(runes) {
							wEnd = len(runes)
						}
						writeln("\033[7m" + string(runes[off:wEnd]) + "\033[0m")
					}
				} else {
					writeln("\033[7m" + plain + "\033[0m")
				}
				for _, ol := range outLines {
					writeln(ol)
				}
			} else {
				writeln(line)
			}
		}
	}

	// Filter bar (no trailing \r\n — cursor stays on this line)
	writeln("")
	b.WriteString(truncateVisible(s.renderFilterBar(), w))

	os.Stdout.WriteString(b.String())
	s.lastLines = lines
}
```

- [ ] **Step 6: Replace `getOutputLines` to use filtered index**

Replace the existing `getOutputLines` method with:

```go
func (s *Selector) getOutputLines(termHeight int) []string {
	if s.cursor >= len(s.filtered) {
		return nil
	}
	entryIdx := s.filtered[s.cursor]
	entry := s.entries[entryIdx]
	text, _ := entry["out"].(string)

	if strings.TrimSpace(text) == "" {
		return []string{Colorize("    (no captured output)", Dim)}
	}

	text = RE_ANSI.ReplaceAllString(text, "")
	text = strings.TrimRight(text, "\n")
	raw := strings.Split(text, "\n")

	maxLines := termHeight / 2
	if maxLines < 1 {
		maxLines = 1
	}

	total := len(raw)
	s.outScroll = max(0, min(s.outScroll, total-maxLines))
	end := min(s.outScroll+maxLines, total)

	var lines []string
	for i := s.outScroll; i < end; i++ {
		lines = append(lines, "    "+raw[i])
	}

	if total > maxLines {
		indicator := fmt.Sprintf("    ── line %d-%d of %d (↑/↓ scroll, Enter close) ──", s.outScroll+1, end, total)
		lines = append(lines, Colorize(indicator, Dim))
	}

	return lines
}
```

- [ ] **Step 7: Verify the display package compiles**

Run: `cd /home/cyb3r/RTLog && go vet ./internal/display/`
Expected: clean (no errors). Note: `go build ./...` will fail because `cmd/show.go` still uses the old `NewSelector` signature — that's fixed in Task 4.

- [ ] **Step 8: Commit**

```bash
git add internal/display/selector.go
git commit -m "feat(display): rewrite Selector for Atuin-style TUI

Replace struct, constructor, input loop, and render method as an atomic
change. Adds filter bar, compact row format, bottom-up display order,
text filtering, tag cycling (Tab), and failed-only toggle (Ctrl+F)."
```

---

### Task 4: Update `cmd/show.go` integration

**Files:**
- Modify: `cmd/show.go`

Remove the manual entry reversal and header construction for the interactive path. Pass entries in chronological order (as returned by DB). Non-interactive paths stay unchanged.

- [ ] **Step 1: Replace everything from line 69 to line 109 in `show.go`**

The current code at lines 69-109 does: build header → compute idxWidth → convert entries → reverse entries → compute origIdx → branch on showOutput/IsTTY/else.

Replace that entire block (from `header := fmt.Sprintf(...)` on line 69 through the closing `}` of the else branch on line 109) with:

```go
		// Convert to display.Entry maps
		entryMaps := make([]display.Entry, len(entries))
		for i, e := range entries {
			entryMaps[i] = logfile.ToMap(e)
		}

		if showOutput {
			// Non-interactive --all: reverse for newest-first, use FmtEntry
			for i, j := 0, len(entryMaps)-1; i < j; i, j = i+1, j-1 {
				entryMaps[i], entryMaps[j] = entryMaps[j], entryMaps[i]
			}
			header := fmt.Sprintf("--- %s ---", logfile.EngagementName(path))
			if dateLabel != "" {
				header += fmt.Sprintf("  [%s]", dateLabel)
			}
			idxWidth := len(fmt.Sprintf("%d", len(entries)))
			n := len(entryMaps)
			origIdx := func(i int) int { return n - i }
			fmt.Println(display.Colorize(header, display.Bold))
			fmt.Println()
			for i, m := range entryMaps {
				fmt.Println(display.FmtEntry(m, origIdx(i), idxWidth, false))
				display.PrintOutputBlock(m, true)
			}
		} else if display.IsTTY {
			// Interactive TUI: entries in chronological order (oldest first)
			sel := display.NewSelector(entryMaps)
			if err := sel.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Non-interactive pipe: reverse for newest-first, use FmtEntry
			for i, j := 0, len(entryMaps)-1; i < j; i, j = i+1, j-1 {
				entryMaps[i], entryMaps[j] = entryMaps[j], entryMaps[i]
			}
			header := fmt.Sprintf("--- %s ---", logfile.EngagementName(path))
			if dateLabel != "" {
				header += fmt.Sprintf("  [%s]", dateLabel)
			}
			idxWidth := len(fmt.Sprintf("%d", len(entries)))
			n := len(entryMaps)
			origIdx := func(i int) int { return n - i }
			fmt.Println(display.Colorize(header, display.Bold))
			fmt.Println()
			for i, m := range entryMaps {
				fmt.Println(display.FmtEntry(m, origIdx(i), idxWidth))
			}
		}
```

Note: the `time` import is still needed (used by date validation at line 25). The `"fmt"` and `"os"` imports are still needed. No import changes required.

- [ ] **Step 2: Verify full project compiles**

Run: `cd /home/cyb3r/RTLog && go build ./...`
Expected: clean compilation, no errors.

- [ ] **Step 3: Run all tests**

Run: `cd /home/cyb3r/RTLog && go test ./...`
Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/show.go
git commit -m "feat(show): wire up Atuin-style TUI selector"
```

---

### Task 5: Manual verification and edge case testing

**Files:** None (testing only)

- [ ] **Step 1: Build the binary**

```bash
cd /home/cyb3r/RTLog && go build -o rtlog .
```

If there's an existing engagement with data, use `./rtlog show` directly. Otherwise create test data:

```bash
./rtlog new test-tui
./rtlog on
./rtlog tag recon
```

- [ ] **Step 2: Verify layout**

Run: `./rtlog show`

Check:
- Newest entry appears at the bottom
- Cursor (highlighted row) starts at the bottom
- Up arrow moves to older entries
- Down arrow moves to newer entries
- Filter bar is visible at the bottom with entry count and `▸ _` prompt

- [ ] **Step 3: Verify filtering**

In the TUI:
- Type a few characters — list should filter instantly
- Backspace should remove characters from filter
- Tab should cycle through tags (if entries have tags)
- Ctrl+F should toggle failed-only filter
- Filter bar should update match count (`N/M matches`)

- [ ] **Step 4: Verify output expansion**

- Press Enter on an entry with `[+out]` — output should appear below the entry
- Up/down should scroll within output when expanded
- Enter again should collapse output

- [ ] **Step 5: Verify edge cases**

- Esc quits the TUI
- Empty filter shows all entries
- Filter that matches nothing shows `(no matches)`
- Non-interactive mode still works: `./rtlog show | cat` (should show old FmtEntry format with header)
- `./rtlog show --all` still works (prints all entries with output, old format)

- [ ] **Step 6: Run full test suite**

Run: `cd /home/cyb3r/RTLog && go test ./...`
Expected: all tests PASS.

- [ ] **Step 7: Final commit (if any fixes needed)**

Only if manual testing revealed issues that required code changes.
