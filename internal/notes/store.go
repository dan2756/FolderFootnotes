// Package notes keeps directory notes and works out which ones apply to a
// directory.
package notes

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
	"unicode/utf8"
)

// Note is the note attached to one directory in the central store.
type Note struct {
	Text    string    `json:"text"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// Store is the central note store: a JSON file of notes keyed by absolute,
// symlink-resolved directory path.
type Store struct {
	Path  string
	Notes map[string]*Note

	lock *os.File // held from LoadLocked until Unlock
}

const storeVersion = 1

type storeFile struct {
	Version int              `json:"version"`
	Notes   map[string]*Note `json:"notes"`
}

// DefaultPath returns where the store lives: $DIRNOTE_FILE if set, otherwise
// $XDG_DATA_HOME/dirnote/notes.json, with XDG_DATA_HOME defaulting to
// ~/.local/share.
func DefaultPath() (string, error) {
	if p := os.Getenv("DIRNOTE_FILE"); p != "" {
		return filepath.Abs(p)
	}
	data := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(data) { // relative values are invalid per the XDG spec
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot locate the note store: %w", err)
		}
		data = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(data, "dirnote", "notes.json"), nil
}

// Load reads the store at path. A missing or empty file is an empty store.
func Load(path string) (*Store, error) {
	s := &Store{Path: path, Notes: map[string]*Note{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return s, nil
	}
	var f storeFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if f.Version > storeVersion {
		return nil, fmt.Errorf("%s: format version %d is newer than this dirnote understands", path, f.Version)
	}
	for dir, n := range f.Notes {
		if n != nil {
			s.Notes[dir] = n
		}
	}
	return s, nil
}

// LoadLocked is Load for a store that is about to be changed. It first
// takes an exclusive lock on path+".lock", waiting while another dirnote
// process holds it, and keeps it until Unlock. Two processes that each load
// the store, change it and save it would otherwise lose one of the changes.
func LoadLocked(path string) (*Store, error) {
	f, err := lockFile(path + ".lock")
	if err != nil {
		return nil, err
	}
	s, err := Load(path)
	if err != nil {
		f.Close()
		return nil, err
	}
	s.lock = f
	return s, nil
}

// Unlock releases the lock taken by LoadLocked, if any.
func (s *Store) Unlock() {
	if s.lock != nil {
		s.lock.Close()
		s.lock = nil
	}
}

// lockFile opens path, creating it and its directory if needed, and waits
// for an exclusive lock on it, which lasts until the file is closed.
func lockFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := flock(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("locking %s: %w", path, err)
	}
	return f, nil
}

// Save writes the store atomically: other processes see either the old file
// or the new one, never a partial write. If the store is a symlink (into a
// dotfiles repository, say) the file it points to is replaced and the link
// stays as it is.
func (s *Store) Save() error {
	for dir := range s.Notes {
		// JSON would silently turn the invalid bytes into U+FFFD, and the
		// note would never be found under its directory again.
		if !utf8.ValidString(dir) {
			return fmt.Errorf("cannot store a note for %q: the path is not valid UTF-8", dir)
		}
	}
	data, err := json.MarshalIndent(storeFile{Version: storeVersion, Notes: s.Notes}, "", "  ")
	if err != nil {
		return err
	}
	path := linkTarget(s.Path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'), 0o600)
}

// Get returns the note for dir.
func (s *Store) Get(dir string) (*Note, bool) {
	n, ok := s.Notes[dir]
	return n, ok
}

// Set replaces the note for dir with text. Times are kept in UTC, to the
// second.
func (s *Store) Set(dir, text string, now time.Time) {
	now = now.UTC().Truncate(time.Second)
	if n, ok := s.Notes[dir]; ok {
		n.Text = text
		n.Updated = now
		return
	}
	s.Notes[dir] = &Note{Text: text, Created: now, Updated: now}
}

// Remove deletes the note for dir and reports whether there was one.
func (s *Store) Remove(dir string) bool {
	_, ok := s.Notes[dir]
	delete(s.Notes, dir)
	return ok
}

// Dirs returns every directory that has a note, sorted.
func (s *Store) Dirs() []string {
	dirs := make([]string, 0, len(s.Notes))
	for d := range s.Notes {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs
}

// Prune removes the notes for which keep returns false and returns their
// directories, sorted.
func (s *Store) Prune(keep func(dir string) bool) []string {
	var gone []string
	for _, dir := range s.Dirs() {
		if !keep(dir) {
			delete(s.Notes, dir)
			gone = append(gone, dir)
		}
	}
	return gone
}
