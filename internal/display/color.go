package display

import (
	"os"
	"regexp"

	"golang.org/x/term"
)

// ANSI color codes.
const (
	Reset   = "\033[0m"
	Cyan    = "\033[36m"
	Green   = "\033[32m"
	Red     = "\033[31m"
	Yellow  = "\033[33m"
	Dim     = "\033[2m"
	Bold    = "\033[1m"
	Magenta = "\033[35m"
)

// IsTTY is true when stdout is a terminal.
var IsTTY = term.IsTerminal(int(os.Stdout.Fd()))

// RE_ANSI matches ANSI escape sequences for stripping.
var RE_ANSI = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]|\x1b\].*?(?:\x07|\x1b\\)|\r`)

// Colorize wraps text with an ANSI color code if stdout is a TTY.
func Colorize(text, code string) string {
	if IsTTY {
		return code + text + Reset
	}
	return text
}
