package display

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// Selector provides an interactive terminal UI for browsing log entries.
type Selector struct {
	entries   []Entry
	header    string
	idxWidth  int
	cursor    int
	offset    int
	expanded  bool
	lastLines int // number of \r\n written on last render
}

// NewSelector creates a Selector for the given entries.
func NewSelector(entries []Entry, header string, idxWidth int) *Selector {
	return &Selector{
		entries:  entries,
		header:   header,
		idxWidth: idxWidth,
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
			}
		} else if n == 3 && buf[0] == 27 && buf[1] == '[' {
			switch buf[2] {
			case 'A':
				s.expanded = false
				s.moveUp()
			case 'B':
				s.expanded = false
				s.moveDown()
			}
		}
	}
}

func (s *Selector) moveDown() {
	if s.cursor < len(s.entries)-1 {
		s.cursor++
	}
}

func (s *Selector) moveUp() {
	if s.cursor > 0 {
		s.cursor--
	}
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
	for _, r := range s {
		if inEsc {
			b.WriteRune(r)
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
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
			break
		}
		b.WriteRune(r)
		vis++
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
	if s.expanded {
		outLines = s.getOutputLines(h)
	}

	visibleLines := h - 4
	if visibleLines < 1 {
		visibleLines = 1
	}

	entrySlots := visibleLines - len(outLines)
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
		line := FmtEntry(s.entries[i], i+1, s.idxWidth)
		if i == s.cursor {
			plain := RE_ANSI.ReplaceAllString(line, "")
			writeln("\033[7m" + plain + "\033[0m")
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
	footer := fmt.Sprintf(" %d/%d  [↑/↓] navigate  [Enter] %s  [q] quit", s.cursor+1, len(s.entries), enterHint)
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

	var lines []string
	for i, l := range raw {
		if i >= maxLines {
			lines = append(lines, Colorize("    ... (truncated)", Dim))
			break
		}
		lines = append(lines, "    "+l)
	}
	return lines
}
