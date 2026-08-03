package notes

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLocalRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if _, ok, err := ReadLocal(dir); ok || err != nil {
		t.Fatalf("no file: ok=%v err=%v", ok, err)
	}
	if err := WriteLocal(dir, "use node 20"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, ".dirnote"))
	if string(b) != "use node 20\n" {
		t.Fatalf("file content %q", b)
	}
	if text, ok, err := ReadLocal(dir); !ok || err != nil || text != "use node 20" {
		t.Fatalf("ReadLocal = %q %v %v", text, ok, err)
	}
	if removed, err := RemoveLocal(dir); !removed || err != nil {
		t.Fatalf("RemoveLocal = %v %v", removed, err)
	}
	if removed, err := RemoveLocal(dir); removed || err != nil {
		t.Fatalf("second RemoveLocal = %v %v", removed, err)
	}
}

func TestReadLocalIgnoresOddFiles(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(LocalPath(dir), []byte("\n\n"), 0o644)
		if _, ok, err := ReadLocal(dir); ok || err != nil {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
	t.Run("directory", func(t *testing.T) {
		dir := t.TempDir()
		os.Mkdir(LocalPath(dir), 0o755)
		if _, ok, err := ReadLocal(dir); ok || err != nil {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
	t.Run("fifo", func(t *testing.T) {
		dir := t.TempDir()
		if err := syscall.Mkfifo(LocalPath(dir), 0o644); err != nil {
			t.Skip("mkfifo:", err)
		}
		done := make(chan struct{})
		go func() {
			defer close(done)
			if _, ok, err := ReadLocal(dir); ok || err != nil {
				t.Errorf("ok=%v err=%v", ok, err)
			}
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("ReadLocal blocked on a FIFO")
		}
	})
	t.Run("too big", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(LocalPath(dir), []byte(strings.Repeat("x", maxLocalSize+1)), 0o644)
		if _, ok, err := ReadLocal(dir); ok || err == nil {
			t.Fatalf("ok=%v err=%v, want an error", ok, err)
		}
	})
}

func TestWriteLocalKeepsMode(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(LocalPath(dir), []byte("old\n"), 0o664)
	os.Chmod(LocalPath(dir), 0o664) // umask may have trimmed it
	if err := WriteLocal(dir, "new"); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(LocalPath(dir))
	if perm := fi.Mode().Perm(); perm != 0o664 {
		t.Fatalf("mode = %o, want 664", perm)
	}
}

func TestCollectBothSources(t *testing.T) {
	dir := tempDir(t)
	st := &Store{Path: "/store.json", Notes: map[string]*Note{}}
	if got, err := Collect(st, dir, Options{}); len(got) != 0 || err != nil {
		t.Fatalf("nothing yet: %v %v", got, err)
	}
	st.Set(dir, "mine", time.Now())
	WriteLocal(dir, "shipped with the repo")
	got, err := Collect(st, dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 ||
		got[0].Source != SourceStore || got[0].Text != "mine" || got[0].File != "/store.json" ||
		got[1].Source != SourceLocal || got[1].Text != "shipped with the repo" || got[1].File != LocalPath(dir) {
		t.Fatalf("Collect = %+v", got)
	}
}
