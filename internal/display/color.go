package display

import (
	"fmt"
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

// isStderrTTY is true when stderr is a terminal (may differ from IsTTY when stdout is piped).
var isStderrTTY = term.IsTerminal(int(os.Stderr.Fd()))

// RE_ANSI matches ANSI escape sequences for stripping.
var RE_ANSI = regexp.MustCompile(`\x1b\[[?>=]*[0-9;]*[A-Za-z]|\x1b\].*?(?:\x07|\x1b\\)|\r`)

// Colorize wraps text with an ANSI color code if stdout is a TTY.
func Colorize(text, code string) string {
	if IsTTY {
		return code + text + Reset
	}
	return text
}

// Warn prints a red warning message to stderr.
func Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if isStderrTTY {
		fmt.Fprintf(os.Stderr, "%swarning: %s%s\n", Red, msg, Reset)
	} else {
		fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
	}
}
