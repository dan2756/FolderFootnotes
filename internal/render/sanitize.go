package render

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Sanitize removes terminal escape sequences and control characters other
// than newline and tab, plus the bidi overrides that can make text display
// differently from what it says. Notes can come from .dirnote files in
// repositories someone else wrote, and must not be able to drive the
// terminal.
func Sanitize(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == '\x1b':
			i += escapeLen(s[i:])
			continue
		case r == '\n', r == '\t':
			b.WriteRune(r)
		case r == utf8.RuneError && size == 1:
			b.WriteRune(utf8.RuneError)
		case unicode.IsControl(r), isBidiControl(r):
			// dropped
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

// escapeLen returns the length of the escape sequence at the start of s,
// which begins with ESC.
func escapeLen(s string) int {
	if len(s) < 2 {
		return len(s)
	}
	switch s[1] {
	case '[': // CSI: parameter and intermediate bytes, then a final byte
		for i := 2; i < len(s); i++ {
			if s[i] >= 0x40 && s[i] <= 0x7e {
				return i + 1
			}
		}
		return len(s)
	case ']', 'P', '_', '^', 'X': // OSC, DCS, APC, PM, SOS: until BEL or ST
		for i := 2; i < len(s); i++ {
			if s[i] == '\a' {
				return i + 1
			}
			if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2
			}
		}
		return len(s)
	}
	return 2
}

func isBidiControl(r rune) bool {
	return (r >= '‪' && r <= '‮') || (r >= '⁦' && r <= '⁩')
}
