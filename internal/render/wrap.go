package render

import (
	"strings"
	"unicode"
)

// Wrap breaks text into lines of at most width columns. Words are kept
// whole unless a single word is wider than the line. A wrapped line keeps
// its indentation, and list items ("- ", "* ", "1. ") get a hanging indent
// so continuation lines line up with the item text.
func Wrap(text string, width int) []string {
	width = max(width, 1)
	var out []string
	for _, l := range lines(text) {
		out = append(out, wrapLine(l, width)...)
	}
	return out
}

// lines splits sanitized text into lines with tabs expanded and trailing
// blanks trimmed.
func lines(text string) []string {
	text = Sanitize(strings.ReplaceAll(text, "\r\n", "\n"))
	text = strings.ReplaceAll(text, "\t", "    ")
	ls := strings.Split(strings.Trim(text, "\n"), "\n")
	for i, l := range ls {
		ls[i] = strings.TrimRight(l, " ")
	}
	return ls
}

func wrapLine(line string, width int) []string {
	if Width(line) <= width {
		return []string{line}
	}
	body := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(body)]
	hang := indent + strings.Repeat(" ", bulletWidth(body))
	if Width(hang) > width/2 {
		indent, hang = "", ""
	}

	var out []string
	cur, curW, empty := indent, Width(indent), true
	for _, word := range strings.Fields(body) {
		ww := Width(word)
		if !empty && curW+1+ww > width {
			out = append(out, cur)
			cur, curW, empty = hang, Width(hang), true
		}
		if !empty {
			cur += " "
			curW++
		}
		for curW+ww > width { // a word that cannot fit on any line
			head, tail := splitWidth(word, width-curW)
			out = append(out, cur+head)
			cur, curW = hang, Width(hang)
			word, ww = tail, ww-Width(head)
		}
		cur += word
		curW += ww
		empty = false
	}
	if !empty {
		out = append(out, cur)
	}
	return out
}

// bulletWidth returns the width of a list marker at the start of s, or 0.
func bulletWidth(s string) int {
	for _, p := range []string{"- ", "* ", "+ ", "• "} {
		if strings.HasPrefix(s, p) {
			return Width(p)
		}
	}
	i := 0
	for i < len(s) && i < 3 && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 && i+1 < len(s) && (s[i] == '.' || s[i] == ')') && s[i+1] == ' ' {
		return i + 2
	}
	return 0
}

// splitWidth splits s after at most n columns, always taking at least one
// rune so callers make progress.
func splitWidth(s string, n int) (string, string) {
	w := 0
	for i, r := range s {
		rw := runeWidth(r)
		if w+rw > n && i > 0 {
			return s[:i], s[i:]
		}
		w += rw
	}
	return s, ""
}

// Width returns how many terminal columns s occupies.
func Width(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

func runeWidth(r rune) int {
	switch {
	case unicode.IsControl(r), unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf):
		return 0
	case isWide(r):
		return 2
	}
	return 1
}

// wide covers the East Asian wide and fullwidth blocks, the common emoji
// blocks and the other emoji with East Asian Width W (✅ ⚡ ☕ ⭐ 🟢 and so
// on), which terminals draw two columns wide.
var wide = [][2]rune{
	{0x1100, 0x115F}, {0x231A, 0x231B}, {0x2329, 0x232A}, {0x23E9, 0x23EC},
	{0x23F0, 0x23F0}, {0x23F3, 0x23F3}, {0x25FD, 0x25FE}, {0x2614, 0x2615},
	{0x2648, 0x2653}, {0x267F, 0x267F}, {0x2693, 0x2693}, {0x26A1, 0x26A1},
	{0x26AA, 0x26AB}, {0x26BD, 0x26BE}, {0x26C4, 0x26C5}, {0x26CE, 0x26CE},
	{0x26D4, 0x26D4}, {0x26EA, 0x26EA}, {0x26F2, 0x26F3}, {0x26F5, 0x26F5},
	{0x26FA, 0x26FA}, {0x26FD, 0x26FD}, {0x2705, 0x2705}, {0x270A, 0x270B},
	{0x2728, 0x2728}, {0x274C, 0x274C}, {0x274E, 0x274E}, {0x2753, 0x2755},
	{0x2757, 0x2757}, {0x2795, 0x2797}, {0x27B0, 0x27B0}, {0x27BF, 0x27BF},
	{0x2B1B, 0x2B1C}, {0x2B50, 0x2B50}, {0x2B55, 0x2B55}, {0x2E80, 0x303E},
	{0x3041, 0x33FF}, {0x3400, 0x4DBF}, {0x4E00, 0x9FFF}, {0xA000, 0xA4CF},
	{0xAC00, 0xD7A3}, {0xF900, 0xFAFF}, {0xFE30, 0xFE4F}, {0xFF00, 0xFF60},
	{0xFFE0, 0xFFE6}, {0x1F004, 0x1F004}, {0x1F0CF, 0x1F0CF}, {0x1F18E, 0x1F18E},
	{0x1F191, 0x1F19A}, {0x1F200, 0x1F202}, {0x1F210, 0x1F23B}, {0x1F240, 0x1F248},
	{0x1F250, 0x1F251}, {0x1F260, 0x1F265}, {0x1F300, 0x1F64F}, {0x1F680, 0x1F6FF},
	{0x1F7E0, 0x1F7EB}, {0x1F7F0, 0x1F7F0}, {0x1F900, 0x1F9FF}, {0x1FA70, 0x1FAFF},
	{0x20000, 0x2FFFD}, {0x30000, 0x3FFFD},
}

func isWide(r rune) bool {
	if r < 0x1100 {
		return false
	}
	for _, rg := range wide {
		if r >= rg[0] && r <= rg[1] {
			return true
		}
	}
	return false
}
