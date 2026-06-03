package notes

import (
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
	s.Append("/src/api", "needs docker", t1)
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
