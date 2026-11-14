package render

import (
	"strings"

	"github.com/rivo/uniseg"
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
// grapheme cluster so callers make progress. Clusters stay whole: a ZWJ
// sequence or a skin-tone emoji is never cut in half.
func splitWidth(s string, n int) (string, string) {
	w := 0
	state := -1
	rest := s
	for len(rest) > 0 {
		_, next, cw, newState := uniseg.FirstGraphemeClusterInString(rest, state)
		if w+cw > n && rest != s {
			return s[:len(s)-len(rest)], rest
		}
		w += cw
		rest = next
		state = newState
	}
	return s, ""
}

// Width returns how many terminal columns s occupies. A grapheme cluster
// (a ZWJ sequence, an emoji with a skin tone, a flag, a character plus
// variation selector) counts as one unit, the way a terminal draws it.
func Width(s string) int {
	return uniseg.StringWidth(s)
}

type grapheme struct {
	text  string
	width int
}

// graphemes returns the grapheme clusters of s with their column widths.
func graphemes(s string) []grapheme {
	var out []grapheme
	state := -1
	for len(s) > 0 {
		cluster, rest, w, newState := uniseg.FirstGraphemeClusterInString(s, state)
		out = append(out, grapheme{cluster, w})
		s, state = rest, newState
	}
	return out
}
