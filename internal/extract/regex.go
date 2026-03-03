package extract

import (
	"regexp"
	"sort"
	"strings"
)

// StringSet is a set of strings backed by a map.
type StringSet map[string]struct{}

// NewStringSet creates an empty StringSet.
func NewStringSet() StringSet {
	return make(StringSet)
}

// Add inserts a value into the set.
func (s StringSet) Add(val string) {
	s[val] = struct{}{}
}

// Has returns true if val is in the set.
func (s StringSet) Has(val string) bool {
	_, ok := s[val]
	return ok
}

// Sorted returns the set's elements in sorted order.
func (s StringSet) Sorted() []string {
	result := make([]string, 0, len(s))
	for k := range s {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

// Len returns the number of elements.
func (s StringSet) Len() int {
	return len(s)
}

// PositionTracker tracks byte positions that have already been claimed
// to prevent cross-pass duplicates in extraction.
type PositionTracker struct {
	seen map[int]struct{}
}

// NewPositionTracker creates a new tracker.
func NewPositionTracker() *PositionTracker {
	return &PositionTracker{seen: make(map[int]struct{})}
}

// Mark records positions in the range [start, end).
func (pt *PositionTracker) Mark(start, end int) {
	for i := start; i < end; i++ {
		pt.seen[i] = struct{}{}
	}
}

// Overlaps returns true if any position in [start, end) is already seen.
func (pt *PositionTracker) Overlaps(start, end int) bool {
	for i := start; i < end; i++ {
		if _, ok := pt.seen[i]; ok {
			return true
		}
	}
	return false
}

// Claim returns true and marks positions if none overlap; returns false otherwise.
func (pt *PositionTracker) Claim(start, end int) bool {
	if pt.Overlaps(start, end) {
		return false
	}
	pt.Mark(start, end)
	return true
}

// BuildFlagRegex builds a compiled regex matching sorted flags (longest-first)
// followed by a value. For Go RE2 (no backreferences), captures \S+ and the
// caller should use StripQuotes on the result.
func BuildFlagRegex(flags []string) *regexp.Regexp {
	sorted := make([]string, len(flags))
	copy(sorted, flags)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})
	escaped := make([]string, len(sorted))
	for i, f := range sorted {
		escaped[i] = regexp.QuoteMeta(f)
	}
	pattern := `(?:^|\s)(?:` + strings.Join(escaped, "|") + `)(?:\s+|=)(\S+)`
	return regexp.MustCompile(pattern)
}

// StripQuotes removes matching leading/trailing single or double quotes.
func StripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') ||
			(s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// IsFileLike returns true if val looks like a file path rather than a credential.
func IsFileLike(val string) bool {
	return strings.Contains(val, "/") ||
		strings.HasSuffix(val, ".txt") ||
		strings.HasSuffix(val, ".lst")
}
