package main

import (
	"os"
	"strconv"

	"golang.org/x/term"
)

// terminalFd returns the file descriptor behind v if it is a terminal.
func terminalFd(v any) (int, bool) {
	f, ok := v.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return 0, false
	}
	return int(f.Fd()), true
}

func isTerminal(v any) bool {
	_, ok := terminalFd(v)
	return ok
}

// useColor reports whether output to w may use ANSI colors: only on a
// terminal, and never when NO_COLOR is set (https://no-color.org) or the
// terminal is dumb.
func useColor(w any) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return isTerminal(w)
}

// termWidth returns the width of the terminal behind w, falling back to
// $COLUMNS and then 80.
func termWidth(w any) int {
	if fd, ok := terminalFd(w); ok {
		if cols, _, err := term.GetSize(fd); err == nil && cols > 0 {
			return cols
		}
	}
	if n, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && n > 0 {
		return n
	}
	return 80
}
