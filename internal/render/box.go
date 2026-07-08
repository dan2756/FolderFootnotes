// Package render lays notes out for the terminal.
package render

import (
	"strings"
)

// Section is one note: a label (usually the directory) and the note text.
type Section struct {
	Label string
	Text  string
}

// Options controls Box.
type Options struct {
	Width int  // columns available, normally the terminal width
	Color bool // dim the labels with ANSI escapes
}

const (
	minInner = 10 // narrowest text area we will draw, whatever the terminal says
	dimOn    = "\x1b[2m"
	dimOff   = "\x1b[22m"
)

// Box draws the sections in one rounded frame, separated by labelled
// dividers, wrapping the text so the frame fits in opt.Width columns:
//
//	╭─ ~/src/api ──────────────────────╮
//	│ run make dev first, needs docker │
//	╰──────────────────────────────────╯
func Box(sections []Section, opt Options) string {
	if len(sections) == 0 {
		return ""
	}
	maxInner := max(opt.Width-4, minInner)

	// Wrap everything first: the frame is as wide as its widest line or
	// label, up to maxInner.
	inner := 0
	labels := make([]string, len(sections))
	bodies := make([][]string, len(sections))
	for i, s := range sections {
		labels[i] = truncateLeft(cleanLabel(s.Label), maxInner-2)
		if labels[i] != "" {
			inner = max(inner, Width(labels[i])+2)
		}
		bodies[i] = Wrap(s.Text, maxInner)
		for _, l := range bodies[i] {
			inner = max(inner, Width(l))
		}
	}

	var b strings.Builder
	for i := range sections {
		left, right := "╭", "╮"
		if i > 0 {
			left, right = "├", "┤"
		}
		b.WriteString(rule(left, right, labels[i], inner, opt.Color))
		for _, l := range bodies[i] {
			b.WriteString("│ ")
			b.WriteString(l)
			b.WriteString(strings.Repeat(" ", inner-Width(l)))
			b.WriteString(" │\n")
		}
	}
	b.WriteString("╰" + strings.Repeat("─", inner+2) + "╯\n")
	return b.String()
}

// rule draws a horizontal border line carrying label.
func rule(left, right, label string, inner int, color bool) string {
	if label == "" {
		return left + strings.Repeat("─", inner+2) + right + "\n"
	}
	fill := inner - Width(label) - 1
	if color {
		label = dimOn + label + dimOff
	}
	return left + "─ " + label + " " + strings.Repeat("─", fill) + right + "\n"
}

// Plain renders the sections without a frame or colors: each label on its
// own line followed by the note, indented.
func Plain(sections []Section) string {
	var b strings.Builder
	for _, s := range sections {
		if l := cleanLabel(s.Label); l != "" {
			b.WriteString(l + ":\n")
		}
		for _, l := range lines(s.Text) {
			b.WriteString(strings.TrimRight("  "+l, " ") + "\n")
		}
	}
	return b.String()
}

// cleanLabel makes a label safe to print on one line.
func cleanLabel(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return ' '
		}
		return r
	}, Sanitize(s))
}

// truncateLeft shortens s to at most n columns by cutting from the left,
// which keeps the informative end of a path.
func truncateLeft(s string, n int) string {
	if Width(s) <= n {
		return s
	}
	rs := []rune(s)
	w := 1 // the ellipsis
	i := len(rs)
	for i > 0 && w+runeWidth(rs[i-1]) <= n {
		i--
		w += runeWidth(rs[i])
	}
	return "…" + string(rs[i:])
}
