package display

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/term"
)

// selectorMode represents the current interaction mode of the Selector.
type selectorMode int

const (
	modeNormal        selectorMode = iota
	modeConfirmDelete              // waiting for y/n
	modeEditChoose                 // waiting for t/n
	modeEditTag                    // editing tag value
	modeEditNote                   // editing note value
)

// ApplyFilters returns indices into entries that match all active filters.
// textFilter is case-insensitive substring across cmd, tool, tag, note, cwd.
// tagFilter is exact match on tag ("" = no filter).
// failOnly filters to non-zero exit codes.
func ApplyFilters(entries []Entry, textFilter, tagFilter string, failOnly bool, useRegex ...bool) []int {
	var result []int
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
		if textFilter != "" {
			fields := []string{
				getString(e, "cmd", ""),
				getString(e, "tool", ""),
				getString(e, "tag", ""),
				getString(e, "note", ""),
				getString(e, "cwd", ""),
			}
			found := false
			if len(useRegex) > 0 && useRegex[0] {
				re, err := regexp.Compile(textFilter)
				if err != nil {
					// Invalid regex: skip text filter (keep all entries visible)
					found = true
				} else {
					for _, f := range fields {
						if re.MatchString(f) {
							found = true
							break
						}
					}
				}
			} else {
				lower := strings.ToLower(textFilter)
				for _, f := range fields {
					if strings.Contains(strings.ToLower(f), lower) {
						found = true
						break
					}
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
	useRegex  bool // regex mode toggle
	filtered  []int    // indices into entries matching current filters
	allTags   []string // unique tags for Tab cycling
	tagIdx    int      // current position in tag cycle (0 = "all")

	// Mutation callbacks (nil = feature disabled)
	OnDelete func(id int64) error
	OnUpdate func(id int64, fields map[string]string) error

	// Mode state
	mode    selectorMode
	editBuf string // buffer for inline editing in modeEditTag/modeEditNote
}

// NewSelector creates a Selector for the given entries (chronological order, oldest first).
func NewSelector(entries []Entry) *Selector {
	s := &Selector{
		entries: entries,
	}
	s.allTags = CollectTags(entries)
	s.filtered = ApplyFilters(entries, "", "", false, false)
	if len(s.filtered) > 0 {
		s.cursor = len(s.filtered) - 1
	}
	return s
}

// applyAndReset rebuilds the filtered slice and resets cursor to newest.
func (s *Selector) applyAndReset() {
	s.filtered = ApplyFilters(s.entries, s.filter, s.tagFilter, s.failOnly, s.useRegex)
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
	switch s.mode {
	case modeConfirmDelete:
		id := s.selectedID()
		return fmt.Sprintf("  Delete entry #%d? (y/n)", id)
	case modeEditChoose:
		return "  Edit: (t)ag or (n)ote?  [Esc cancel]"
	case modeEditTag:
		return fmt.Sprintf("  Tag: %s_  [Enter save, Esc cancel]", s.editBuf)
	case modeEditNote:
		return fmt.Sprintf("  Note: %s_  [Enter save, Esc cancel]", s.editBuf)
	}

	// modeNormal — original filter bar
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

	if s.useRegex {
		if s.filter != "" {
			if _, err := regexp.Compile(s.filter); err != nil {
				parts = append(parts, fmt.Sprintf("▸ /%s/_ %s", s.filter, Colorize("[invalid regex]", Red)))
			} else {
				parts = append(parts, fmt.Sprintf("▸ /%s/_ %s", s.filter, Colorize("[regex]", Cyan)))
			}
		} else {
			parts = append(parts, fmt.Sprintf("▸ //_  %s", Colorize("[regex]", Cyan)))
		}
	} else {
		parts = append(parts, fmt.Sprintf("▸ %s_", s.filter))
	}

	return "  " + strings.Join(parts, "   ")
}

// selectedID returns the database ID of the currently selected entry, or -1.
func (s *Selector) selectedID() int64 {
	if len(s.filtered) == 0 || s.cursor >= len(s.filtered) {
		return -1
	}
	entry := s.entries[s.filtered[s.cursor]]
	id, ok := entry["id"].(int64)
	if !ok {
		return -1
	}
	return id
}

// handleDelete processes the delete confirmation.
func (s *Selector) handleDelete() {
	id := s.selectedID()
	if id < 0 || s.OnDelete == nil {
		s.mode = modeNormal
		return
	}
	if err := s.OnDelete(id); err != nil {
		s.mode = modeNormal
		return
	}
	// Splice entry out of s.entries
	entryIdx := s.filtered[s.cursor]
	s.entries = append(s.entries[:entryIdx], s.entries[entryIdx+1:]...)
	// Rebuild filtered list and adjust cursor
	s.filtered = ApplyFilters(s.entries, s.filter, s.tagFilter, s.failOnly, s.useRegex)
	s.allTags = CollectTags(s.entries)
	if s.cursor >= len(s.filtered) {
		s.cursor = len(s.filtered) - 1
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
	s.expanded = false
	s.mode = modeNormal
}

// handleEditSave persists the edit buffer to the database and updates in-memory state.
func (s *Selector) handleEditSave() {
	id := s.selectedID()
	if id < 0 || s.OnUpdate == nil {
		s.mode = modeNormal
		return
	}

	var field string
	if s.mode == modeEditTag {
		field = "tag"
	} else {
		field = "note"
	}

	if err := s.OnUpdate(id, map[string]string{field: s.editBuf}); err != nil {
		s.mode = modeNormal
		return
	}

	// Update in-memory entry
	entryIdx := s.filtered[s.cursor]
	s.entries[entryIdx][field] = s.editBuf

	// Rebuild filters in case the edit affects visible entries
	s.filtered = ApplyFilters(s.entries, s.filter, s.tagFilter, s.failOnly, s.useRegex)
	s.allTags = CollectTags(s.entries)
	if s.cursor >= len(s.filtered) {
		s.cursor = len(s.filtered) - 1
	}
	if s.cursor < 0 {
		s.cursor = 0
	}

	s.mode = modeNormal
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
			switch s.mode {
			case modeConfirmDelete:
				if buf[0] == 'y' || buf[0] == 'Y' {
					s.handleDelete()
				} else {
					s.mode = modeNormal
				}
				continue

			case modeEditChoose:
				switch buf[0] {
				case 't', 'T':
					s.editBuf = getString(s.entries[s.filtered[s.cursor]], "tag", "")
					s.mode = modeEditTag
				case 'n', 'N':
					s.editBuf = getString(s.entries[s.filtered[s.cursor]], "note", "")
					s.mode = modeEditNote
				case 27: // Esc
					s.mode = modeNormal
				default:
					s.mode = modeNormal
				}
				continue

			case modeEditTag, modeEditNote:
				switch buf[0] {
				case 27: // Esc
					s.mode = modeNormal
				case 13: // Enter — save
					s.handleEditSave()
				case 127, 8: // Backspace
					if len(s.editBuf) > 0 {
						runes := []rune(s.editBuf)
						s.editBuf = string(runes[:len(runes)-1])
					}
				default:
					if buf[0] >= 0x20 && buf[0] <= 0x7E {
						s.editBuf += string(buf[0])
					}
				}
				continue
			}

			// modeNormal handlers
			switch buf[0] {
			case 27: // Esc
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
			case 4: // Ctrl+D — delete
				if len(s.filtered) > 0 && s.OnDelete != nil {
					s.mode = modeConfirmDelete
				}
			case 5: // Ctrl+E — edit
				if len(s.filtered) > 0 && s.OnUpdate != nil {
					s.mode = modeEditChoose
				}
			case 6: // Ctrl+F — toggle failed only
				s.failOnly = !s.failOnly
				s.applyAndReset()
			case 18: // Ctrl+R — toggle regex mode
				s.useRegex = !s.useRegex
				s.applyAndReset()
			case 127, 8: // Backspace
				if len(s.filter) > 0 {
					runes := []rune(s.filter)
					s.filter = string(runes[:len(runes)-1])
					s.applyAndReset()
				}
			default:
				if buf[0] >= 0x20 && buf[0] <= 0x7E {
					s.filter += string(buf[0])
					s.applyAndReset()
				}
			}
		}
	}
}

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

func (s *Selector) scrollOutDown() {
	s.outScroll++
}

func (s *Selector) scrollOutUp() {
	if s.outScroll > 0 {
		s.outScroll--
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

	var outLines []string
	var wrapExtra int
	if s.expanded && len(s.filtered) > 0 {
		outLines = s.getOutputLines(h)
		entryIdx := s.filtered[s.cursor]
		curLine := RE_ANSI.ReplaceAllString(FmtCompact(s.entries[entryIdx], w), "")
		nRunes := len([]rune(curLine))
		if nRunes > w {
			wrapExtra = (nRunes - 1) / w
		}
	}

	entrySlots := h - 3 - len(outLines) - wrapExtra
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
			line := FmtCompact(s.entries[entryIdx], w)
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

	writeln("")
	if s.mode == modeNormal {
		hint := Colorize("  Enter expand  Tab tag  ^D del  ^E edit  ^R regex  ^F fail  Esc quit", Dim)
		writeln(hint)
	}
	b.WriteString(truncateVisible(s.renderFilterBar(), w))

	os.Stdout.WriteString(b.String())
	s.lastLines = lines
}

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
