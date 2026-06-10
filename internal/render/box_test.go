package render

import (
	"reflect"
	"strings"
	"testing"
)

func TestWrap(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{"fits", "short line", 20, []string{"short line"}},
		{"words", "run make dev first, needs docker", 12,
			[]string{"run make dev", "first, needs", "docker"}},
		{"keeps blank lines", "a\n\nb\n", 10, []string{"a", "", "b"}},
		{"long word", "see https://example.com/a/very/long/path ok", 10,
			[]string{"see", "https://ex", "ample.com/", "a/very/lon", "g/path ok"}},
		{"indent", "    indented text here", 14,
			[]string{"    indented", "    text here"}},
		{"bullet", "- first item wraps here\n- second", 12,
			[]string{"- first item", "  wraps here", "- second"}},
		{"numbered", "10. start the db container", 14,
			[]string{"10. start the", "    db", "    container"}},
		{"tabs and CRLF", "a\tb\r\nc", 20, []string{"a    b", "c"}},
		{"wide runes", "日本語のテキスト", 6, []string{"日本語", "のテキ", "スト"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Wrap(tt.text, tt.width)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Wrap(%q, %d)\n got %q\nwant %q", tt.text, tt.width, got, tt.want)
			}
			for _, l := range got {
				if Width(l) > tt.width {
					t.Errorf("line %q is wider than %d", l, tt.width)
				}
			}
		})
	}
}

func TestBox(t *testing.T) {
	got := Box([]Section{{Label: "~/src/api", Text: "run make dev first, needs docker"}}, Options{Width: 80})
	want := "" +
		"╭─ ~/src/api ──────────────────────╮\n" +
		"│ run make dev first, needs docker │\n" +
		"╰──────────────────────────────────╯\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestBoxWrapsToWidth(t *testing.T) {
	text := "run make dev first, it needs docker running and a .env copied from the template"
	got := Box([]Section{{Label: "~/src/api", Text: text}}, Options{Width: 30})
	want := "" +
		"╭─ ~/src/api ────────────────╮\n" +
		"│ run make dev first, it     │\n" +
		"│ needs docker running and a │\n" +
		"│ .env copied from the       │\n" +
		"│ template                   │\n" +
		"╰────────────────────────────╯\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	for _, l := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
		if w := Width(l); w > 30 {
			t.Errorf("line %q is %d columns, more than 30", l, w)
		}
	}
}

func TestBoxSections(t *testing.T) {
	got := Box([]Section{
		{Label: "~/src/api", Text: "one"},
		{Label: "~/src/api/.dirnote", Text: "two"},
	}, Options{Width: 80})
	want := "" +
		"╭─ ~/src/api ──────────╮\n" +
		"│ one                  │\n" +
		"├─ ~/src/api/.dirnote ─┤\n" +
		"│ two                  │\n" +
		"╰──────────────────────╯\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestBoxLongLabel(t *testing.T) {
	got := Box([]Section{{Label: "/a/very/long/path/to/some/project", Text: "x"}}, Options{Width: 20})
	want := "" +
		"╭─ …/some/project ─╮\n" +
		"│ x                │\n" +
		"╰──────────────────╯\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestBoxColor(t *testing.T) {
	got := Box([]Section{{Label: "~/x", Text: "note"}}, Options{Width: 80, Color: true})
	if !strings.Contains(got, "\x1b[2m~/x\x1b[22m") {
		t.Fatalf("label should be dimmed: %q", got)
	}
	plain := Box([]Section{{Label: "~/x", Text: "note"}}, Options{Width: 80})
	if strings.Contains(plain, "\x1b") {
		t.Fatalf("no escapes expected without Color: %q", plain)
	}
	// Escapes must not change the layout.
	strip := strings.NewReplacer(dimOn, "", dimOff, "")
	if strip.Replace(got) != plain {
		t.Fatalf("colored box differs from plain box:\n%s\n%s", got, plain)
	}
}

func TestPlain(t *testing.T) {
	got := Plain([]Section{{Label: "~/src/api", Text: "one\ntwo"}, {Label: "~/src", Text: "three"}})
	want := "~/src/api:\n  one\n  two\n~/src:\n  three\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWidth(t *testing.T) {
	for s, want := range map[string]int{
		"abc": 3,
		"日本":  4,
		"é":  1, // combining accent
		"👍":   2,
		"│ ─": 3,
	} {
		if got := Width(s); got != want {
			t.Errorf("Width(%q) = %d, want %d", s, got, want)
		}
	}
}
