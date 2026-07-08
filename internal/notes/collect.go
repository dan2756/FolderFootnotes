package notes

import "time"

// Source says where a note came from.
type Source string

const (
	SourceStore Source = "store" // the central notes.json
	SourceLocal Source = "local" // a .dirnote file in the directory
)

// Entry is one note that applies to a directory.
type Entry struct {
	Dir     string // the directory the note belongs to
	Source  Source
	File    string // the file the note was read from
	Text    string
	Created time.Time // store notes only
	Updated time.Time // store notes only
}

// Collect returns the notes for dir: its entry in the store, then its
// .dirnote file.
func Collect(st *Store, dir string) ([]Entry, error) {
	var out []Entry
	if n, ok := st.Get(dir); ok {
		out = append(out, Entry{Dir: dir, Source: SourceStore, File: st.Path, Text: n.Text, Created: n.Created, Updated: n.Updated})
	}
	text, ok, err := ReadLocal(dir)
	if err != nil {
		return out, err
	}
	if ok {
		out = append(out, Entry{Dir: dir, Source: SourceLocal, File: LocalPath(dir), Text: text})
	}
	return out, nil
}
