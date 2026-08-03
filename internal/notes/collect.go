package notes

import (
	"path/filepath"
	"strings"
	"time"
)

// Source says where a note came from.
type Source string

const (
	SourceStore Source = "store" // the central notes.json
	SourceLocal Source = "local" // a .dirnote file in the directory
)

// Entry is one note that applies to a directory.
type Entry struct {
	Dir       string // the directory the note belongs to
	Source    Source
	File      string // the file the note was read from
	Text      string
	Created   time.Time // store notes only
	Updated   time.Time // store notes only
	Inherited bool      // Dir is an ancestor of the directory asked about
}

// Options controls Collect.
type Options struct {
	// Inherit adds the notes of ancestor directories, closest first.
	Inherit bool

	// From is the directory the shell was in before the cd, if known. When
	// it equals the directory asked about nothing has changed and nothing is
	// returned. Otherwise an ancestor's notes are only included when From is
	// outside that ancestor's subtree: entering a project shows its note,
	// moving around inside it does not.
	From string
}

// Collect returns the notes to show for dir: its entry in the store, then
// its .dirnote file, then (with Inherit) the same for each ancestor.
// Unreadable .dirnote files are reported in err without stopping the walk.
func Collect(st *Store, dir string, opt Options) ([]Entry, error) {
	if opt.From != "" && opt.From == dir {
		return nil, nil
	}
	out, err := collectDir(st, dir)
	if !opt.Inherit {
		return out, err
	}
	for a := parent(dir); a != ""; a = parent(a) {
		if opt.From != "" && Within(opt.From, a) {
			// Already inside a's subtree, and so inside every ancestor
			// above it too.
			break
		}
		es, e := collectDir(st, a)
		for i := range es {
			es[i].Inherited = true
		}
		out = append(out, es...)
		if err == nil {
			err = e
		}
	}
	return out, err
}

func collectDir(st *Store, dir string) ([]Entry, error) {
	var out []Entry
	if n, ok := st.Get(dir); ok {
		out = append(out, Entry{Dir: dir, Source: SourceStore, File: st.Path, Text: n.Text, Created: n.Created, Updated: n.Updated})
	}
	text, ok, err := ReadLocal(dir)
	if ok {
		out = append(out, Entry{Dir: dir, Source: SourceLocal, File: LocalPath(dir), Text: text})
	}
	return out, err
}

// parent returns the parent of dir, or "" at the root.
func parent(dir string) string {
	p := filepath.Dir(dir)
	if p == dir {
		return ""
	}
	return p
}

// Within reports whether path is dir or somewhere below it. Both must be
// clean absolute paths.
func Within(path, dir string) bool {
	if path == dir {
		return true
	}
	if !strings.HasSuffix(dir, string(filepath.Separator)) {
		dir += string(filepath.Separator)
	}
	return strings.HasPrefix(path, dir)
}
