package notes

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "notes.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Notes) != 0 {
		t.Fatalf("missing store should be empty, got %v", s.Notes)
	}

	t0 := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	s.Set("/src/api", "run make dev first", t0)
	s.Set("/src/api", "run make dev first\nneeds docker", t1)
	s.Set("/src/web", "pnpm, not npm", t0)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]*Note{
		"/src/api": {Text: "run make dev first\nneeds docker", Created: t0, Updated: t1},
		"/src/web": {Text: "pnpm, not npm", Created: t0, Updated: t0},
	}
	if !reflect.DeepEqual(got.Notes, want) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got.Notes, want)
	}
	if dirs := got.Dirs(); !reflect.DeepEqual(dirs, []string{"/src/api", "/src/web"}) {
		t.Fatalf("Dirs() = %v", dirs)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("store permissions = %o, want 600", perm)
	}
}

func TestSetNormalizesTime(t *testing.T) {
	s := &Store{Notes: map[string]*Note{}}
	zone := time.FixedZone("CEST", 2*60*60)
	s.Set("/a", "x", time.Date(2026, 8, 1, 21, 30, 15, 987654321, zone))
	want := time.Date(2026, 8, 1, 19, 30, 15, 0, time.UTC)
	if n := s.Notes["/a"]; !n.Created.Equal(want) || n.Created.Location() != time.UTC || n.Created.Nanosecond() != 0 {
		t.Fatalf("created = %v, want %v", n.Created, want)
	}
}

func TestStoreRemove(t *testing.T) {
	s := &Store{Notes: map[string]*Note{}}
	s.Set("/a", "x", time.Now())
	if !s.Remove("/a") {
		t.Fatal("Remove of existing note returned false")
	}
	if s.Remove("/a") {
		t.Fatal("Remove of missing note returned true")
	}
}

func TestSaveLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	s, _ := Load(filepath.Join(dir, "notes.json"))
	for i := 0; i < 3; i++ {
		s.Set("/a", strings.Repeat("x", i+1), time.Now())
		if err := s.Save(); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "notes.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("directory contains %v, want only notes.json", names)
	}
}

func TestLoadErrors(t *testing.T) {
	dir := t.TempDir()

	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte("{not json"), 0o600)
	if _, err := Load(bad); err == nil || !strings.Contains(err.Error(), bad) {
		t.Errorf("corrupt store: err = %v, want an error naming the file", err)
	}

	future := filepath.Join(dir, "future.json")
	os.WriteFile(future, []byte(`{"version": 99, "notes": {}}`), 0o600)
	if _, err := Load(future); err == nil {
		t.Error("newer format version should be rejected")
	}

	empty := filepath.Join(dir, "empty.json")
	os.WriteFile(empty, []byte("\n"), 0o600)
	if s, err := Load(empty); err != nil || len(s.Notes) != 0 {
		t.Errorf("empty file: %v, %v", s, err)
	}
}

func TestDefaultPath(t *testing.T) {
	t.Setenv("DIRNOTE_FILE", "")
	t.Setenv("HOME", "/home/u")
	t.Setenv("XDG_DATA_HOME", "")
	if p, _ := DefaultPath(); p != "/home/u/.local/share/dirnote/notes.json" {
		t.Errorf("default = %q", p)
	}
	t.Setenv("XDG_DATA_HOME", "relative/ignored")
	if p, _ := DefaultPath(); p != "/home/u/.local/share/dirnote/notes.json" {
		t.Errorf("relative XDG_DATA_HOME = %q", p)
	}
	t.Setenv("XDG_DATA_HOME", "/data")
	if p, _ := DefaultPath(); p != "/data/dirnote/notes.json" {
		t.Errorf("XDG_DATA_HOME = %q", p)
	}
	t.Setenv("DIRNOTE_FILE", "/elsewhere/n.json")
	if p, _ := DefaultPath(); p != "/elsewhere/n.json" {
		t.Errorf("DIRNOTE_FILE = %q", p)
	}
}

func TestPrune(t *testing.T) {
	s := &Store{Notes: map[string]*Note{}}
	for _, d := range []string{"/keep", "/gone/b", "/gone/a"} {
		s.Set(d, "x", time.Now())
	}
	gone := s.Prune(func(dir string) bool { return dir == "/keep" })
	if !reflect.DeepEqual(gone, []string{"/gone/a", "/gone/b"}) {
		t.Fatalf("Prune returned %v", gone)
	}
	if !reflect.DeepEqual(s.Dirs(), []string{"/keep"}) {
		t.Fatalf("left %v", s.Dirs())
	}
}

func TestSaveThroughSymlink(t *testing.T) {
	dotfiles := t.TempDir()
	target := filepath.Join(dotfiles, "notes.json")
	os.WriteFile(target, []byte(`{"version":1,"notes":{}}`), 0o644)
	os.Chmod(target, 0o644) // whatever the umask
	link := filepath.Join(t.TempDir(), "notes.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	s, err := Load(link)
	if err != nil {
		t.Fatal(err)
	}
	s.Set("/src/api", "via the link", time.Now())
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("store symlink was replaced: %v %v", fi.Mode(), err)
	}
	got, err := Load(target)
	if err != nil || got.Notes["/src/api"] == nil {
		t.Fatalf("note did not reach the link target: %v %+v", err, got)
	}
	if fi, _ := os.Stat(target); fi.Mode().Perm() != 0o644 {
		t.Errorf("target mode changed to %o", fi.Mode().Perm())
	}
}

func TestSaveThroughDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "notes.json")
	if err := os.Symlink("sub/actual.json", link); err != nil {
		t.Fatal(err)
	}
	s, _ := Load(link)
	s.Set("/a", "x", time.Now())
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sub", "actual.json")); err != nil {
		t.Fatalf("link target not created: %v", err)
	}
}

func TestSaveThroughRelativeSymlinkInSymlinkedDir(t *testing.T) {
	// ~/.local/share/dirnote -> ~/data/dirnote, whose notes.json is
	// ../dotfiles/notes.json: the ".." starts from ~/data/dirnote.
	home := t.TempDir()
	data := filepath.Join(home, "data")
	for _, d := range []string{"data/dirnote", "data/dotfiles", ".local/share"} {
		os.MkdirAll(filepath.Join(home, d), 0o755)
	}
	target := filepath.Join(data, "dotfiles", "notes.json")
	os.WriteFile(target, []byte(`{"version":1,"notes":{}}`), 0o600)
	share := filepath.Join(home, ".local", "share")
	if err := os.Symlink(filepath.Join(data, "dirnote"), filepath.Join(share, "dirnote")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../dotfiles/notes.json", filepath.Join(data, "dirnote", "notes.json")); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(share, "dirnote", "notes.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Set("/src/api", "via two links", time.Now())
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(target); err != nil || got.Notes["/src/api"] == nil {
		t.Fatalf("note did not reach the link target: %v %+v", err, got)
	}
	if got, err := Load(path); err != nil || got.Notes["/src/api"] == nil {
		t.Fatalf("note not found through the store path: %v %+v", err, got)
	}
	if _, err := os.Stat(filepath.Join(share, "dotfiles")); err == nil {
		t.Error("save created a stray file next to the lexical path")
	}
}

func TestLoadLockedSerializesUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	const n = 20
	errs := make(chan error, n)
	for i := range n {
		go func() {
			s, err := LoadLocked(path)
			if err != nil {
				errs <- err
				return
			}
			defer s.Unlock()
			s.Set(fmt.Sprintf("/dir/%d", i), "x", time.Now())
			errs <- s.Save()
		}()
	}
	for range n {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Notes) != n {
		t.Fatalf("store has %d notes, want %d: concurrent updates were lost", len(s.Notes), n)
	}
}

func TestSaveRejectsInvalidUTF8(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	s, _ := Load(path)
	s.Set("/home/u/caf\xe9", "latin-1 name", time.Now())
	if err := s.Save(); err == nil {
		t.Fatal("a path that JSON cannot keep should be refused")
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("store written anyway")
	}
}
