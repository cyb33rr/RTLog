package display

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"golang.org/x/term"
)

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

// Selector provides an interactive terminal UI for browsing log entries.
type Selector struct {
	entries   []Entry
	header    string
	idxWidth  int
	cursor    int
	offset    int
	expanded  bool
	reversed  bool // true = newest first (default)
	outScroll int  // scroll offset within expanded output
	lastLines int  // number of \r\n written on last render
}

// NewSelector creates a Selector for the given entries.
func NewSelector(entries []Entry, header string, idxWidth int) *Selector {
	return &Selector{
		entries:  entries,
		header:   header,
		idxWidth: idxWidth,
		reversed: true,
	}
}

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

		if n == 1 {
			switch buf[0] {
			case 'q', 27:
				return nil
			case 13:
				s.expanded = !s.expanded
				s.outScroll = 0
			case 'r':
				for i, j := 0, len(s.entries)-1; i < j; i, j = i+1, j-1 {
					s.entries[i], s.entries[j] = s.entries[j], s.entries[i]
				}
				s.reversed = !s.reversed
				s.cursor = 0
				s.offset = 0
				s.outScroll = 0
				s.expanded = false
			}
		} else if n == 3 && buf[0] == 27 && buf[1] == '[' {
			switch buf[2] {
			case 'A':
				if s.expanded {
					s.scrollOutUp()
				} else {
					s.moveUp()
				}
			case 'B':
				if s.expanded {
					s.scrollOutDown()
				} else {
					s.moveDown()
				}
			}
		}
	}
}

func (s *Selector) moveDown() {
	if s.cursor < len(s.entries)-1 {
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

func (s *Selector) scrollOutDown() {
	s.outScroll++
}

func (s *Selector) scrollOutUp() {
	if s.outScroll > 0 {
		s.outScroll--
	}
}

// origIdx returns the original 1-based entry number for position i.
func (s *Selector) origIdx(i int) int {
	if s.reversed {
		return len(s.entries) - i
	}
	return i + 1
}

func (s *Selector) clearPrev() {
	if s.lastLines > 0 {
		// Move to column 0, then up N lines, then clear to end of screen
		fmt.Fprintf(os.Stdout, "\r\033[%dA\033[J", s.lastLines)
		s.lastLines = 0
	}
}

// truncateVisible truncates a string (which may contain ANSI codes) so visible
// characters don't exceed width. This prevents terminal line wrapping.
func truncateVisible(s string, width int) string {
	if width <= 0 {
		return s
	}
	var b strings.Builder
	vis := 0
	inEsc := false
	inOSC := false
	prevEsc := false // tracks if previous rune was \033, used to detect ST (\x1b\)
	truncated := false
	for _, r := range s {
		if inOSC {
			b.WriteRune(r)
			if prevEsc && r == '\\' {
				// ST terminator: \x1b\ ends the OSC
				inOSC = false
				prevEsc = false
			} else if r == '\x07' {
				// BEL terminator ends the OSC
				inOSC = false
				prevEsc = false
			} else {
				prevEsc = r == '\033'
			}
			continue
		}
		if inEsc {
			b.WriteRune(r)
			if r == ']' {
				// Switch to OSC mode
				inEsc = false
				inOSC = true
				prevEsc = false
			} else if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		if r == '\033' {
			inEsc = true
			b.WriteRune(r)
			continue
		}
		if vis >= width {
			truncated = true
			break
		}
		b.WriteRune(r)
		vis++
	}
	// If truncated mid-string, close any unclosed ANSI sequences
	if truncated {
		b.WriteString(Reset)
	}
	return b.String()
}

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

	// Header
	writeln(Colorize(s.header, Bold))
	writeln("")

	var outLines []string
	var wrapExtra int
	if s.expanded {
		outLines = s.getOutputLines(h)
		// Calculate extra lines from wrapping the selected entry
		curLine := RE_ANSI.ReplaceAllString(FmtEntry(s.entries[s.cursor], s.origIdx(s.cursor), s.idxWidth), "")
		nRunes := len([]rune(curLine))
		if nRunes > w {
			wrapExtra = (nRunes - 1) / w
		}
	}

	visibleLines := h - 4
	if visibleLines < 1 {
		visibleLines = 1
	}

	entrySlots := visibleLines - len(outLines) - wrapExtra
	if entrySlots > 10 {
		entrySlots = 10
	}
	if entrySlots < 1 {
		entrySlots = 1
	}

	if s.cursor < s.offset {
		s.offset = s.cursor
	}
	if s.cursor >= s.offset+entrySlots {
		s.offset = s.cursor - entrySlots + 1
	}

	end := s.offset + entrySlots
	if end > len(s.entries) {
		end = len(s.entries)
	}

	for i := s.offset; i < end; i++ {
		line := FmtEntry(s.entries[i], s.origIdx(i), s.idxWidth)
		if i == s.cursor {
			plain := RE_ANSI.ReplaceAllString(line, "")
			if s.expanded {
				// Wrap full text across multiple lines
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

	// Footer (no trailing \r\n — cursor stays on this line)
	writeln("")
	enterHint := "view output"
	if s.expanded {
		enterHint = "hide output"
	}
	curPos := s.origIdx(s.cursor)
	if len(s.entries) == 0 {
		curPos = 0
	}
	orderHint := "oldest first"
	if s.reversed {
		orderHint = "newest first"
	}
	footer := fmt.Sprintf(" %d/%d  [↑/↓] navigate  [Enter] %s  [r] reverse (%s)  [q] quit", curPos, len(s.entries), enterHint, orderHint)
	b.WriteString(truncateVisible(Colorize(footer, Dim), w))
	// Don't add \r\n for the last line — lines count is for how many to move UP

	os.Stdout.WriteString(b.String())
	s.lastLines = lines
}

func (s *Selector) getOutputLines(termHeight int) []string {
	entry := s.entries[s.cursor]
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

	// Clamp scroll offset
	total := len(raw)
	s.outScroll = max(0, min(s.outScroll, total-maxLines))
	end := min(s.outScroll+maxLines, total)

	var lines []string
	for i := s.outScroll; i < end; i++ {
		lines = append(lines, "    "+raw[i])
	}

	// Scroll indicator
	if total > maxLines {
		indicator := fmt.Sprintf("    ── line %d-%d of %d (↑/↓ scroll, Enter close) ──", s.outScroll+1, end, total)
		lines = append(lines, Colorize(indicator, Dim))
	}

	return lines
}
