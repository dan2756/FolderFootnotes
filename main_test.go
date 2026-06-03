package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type result struct {
	code           int
	stdout, stderr string
}

// setup points the store at a temp file and returns a resolved temp dir to
// attach notes to.
func setup(t *testing.T) string {
	t.Helper()
	t.Setenv("DIRNOTE_FILE", filepath.Join(t.TempDir(), "notes.json"))
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func runCLI(t *testing.T, stdin string, args ...string) result {
	t.Helper()
	var out, errb bytes.Buffer
	c := &cli{
		stdin:  strings.NewReader(stdin),
		stdout: &out,
		stderr: &errb,
		now:    func() time.Time { return time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC) },
	}
	code := c.run(args)
	return result{code, out.String(), errb.String()}
}

func TestAddShowRm(t *testing.T) {
	dir := setup(t)

	if r := runCLI(t, "", "show", "-d", dir); r.code != exitNoNote {
		t.Fatalf("show before add: %+v", r)
	}
	if r := runCLI(t, "", "add", "-d", dir, "run", "make", "dev", "first"); r.code != 0 {
		t.Fatalf("add: %+v", r)
	}
	// flags may come after the text
	if r := runCLI(t, "", "add", "needs docker", "-d", dir); r.code != 0 {
		t.Fatalf("add: %+v", r)
	}
	r := runCLI(t, "", "show", "-d", dir)
	if r.code != 0 || r.stdout != "run make dev first\nneeds docker\n" {
		t.Fatalf("show: %+v", r)
	}

	if r := runCLI(t, "", "set", "-d", dir, "just one line"); r.code != 0 {
		t.Fatalf("set: %+v", r)
	}
	if r := runCLI(t, "", "-d", dir); r.stdout != "just one line\n" {
		t.Fatalf("default command should be show: %+v", r)
	}

	if r := runCLI(t, "", "rm", "-d", dir); r.code != 0 {
		t.Fatalf("rm: %+v", r)
	}
	if r := runCLI(t, "", "rm", "-d", dir); r.code != exitNoNote {
		t.Fatalf("second rm: %+v", r)
	}
}

func TestAddFromStdin(t *testing.T) {
	dir := setup(t)
	if r := runCLI(t, "line one\nline two\n\n", "add", "-d", dir); r.code != 0 {
		t.Fatalf("add: %+v", r)
	}
	if r := runCLI(t, "", "show", "-d", dir); r.stdout != "line one\nline two\n" {
		t.Fatalf("show: %q", r.stdout)
	}
	if r := runCLI(t, "  \n", "add", "-d", dir); r.code != exitError {
		t.Fatalf("empty add should fail: %+v", r)
	}
}

func TestDashDash(t *testing.T) {
	dir := setup(t)
	runCLI(t, "", "set", "-d", dir, "--", "-v", "is", "verbose")
	if r := runCLI(t, "", "show", "-d", dir); r.stdout != "-v is verbose\n" {
		t.Fatalf("show: %+v", r)
	}
}

func TestSymlinkedDirSharesNote(t *testing.T) {
	dir := setup(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	runCLI(t, "", "add", "-d", link, "via the link")
	if r := runCLI(t, "", "show", "-d", dir); r.stdout != "via the link\n" {
		t.Fatalf("show: %+v", r)
	}
}

func TestLsFlagsMissing(t *testing.T) {
	dir := setup(t)
	gone := filepath.Join(dir, "gone")
	os.Mkdir(gone, 0o755)
	runCLI(t, "", "add", "-d", dir, "kept")
	runCLI(t, "", "add", "-d", gone, "deleted later")
	os.Remove(gone)

	r := runCLI(t, "", "ls")
	if r.code != 0 {
		t.Fatalf("ls: %+v", r)
	}
	want := tildify(dir) + "\n  kept\n" + tildify(gone) + " (missing)\n  deleted later\n"
	if r.stdout != want {
		t.Fatalf("ls output:\n%s\nwant:\n%s", r.stdout, want)
	}
}

func TestAddRequiresDirectory(t *testing.T) {
	dir := setup(t)
	if r := runCLI(t, "", "add", "-d", filepath.Join(dir, "nope"), "x"); r.code != exitError {
		t.Fatalf("add to missing dir: %+v", r)
	}
}

func TestUsageErrors(t *testing.T) {
	setup(t)
	for _, args := range [][]string{
		{"frobnicate"},
		{"show", "--bogus"},
		{"rm", "extra"},
	} {
		if r := runCLI(t, "", args...); r.code != exitError || r.stderr == "" {
			t.Errorf("%v: %+v", args, r)
		}
	}
	if r := runCLI(t, "", "add", "-h"); r.code != 0 || !strings.Contains(r.stdout, "Usage: dirnote add") {
		t.Errorf("add -h: %+v", r)
	}
	if r := runCLI(t, "", "--version"); r.code != 0 || !strings.HasPrefix(r.stdout, "dirnote ") {
		t.Errorf("--version: %+v", r)
	}
}

func TestTildify(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	for in, want := range map[string]string{
		"/home/u":         "~",
		"/home/u/src/api": "~/src/api",
		"/home/user":      "/home/user",
		"/tmp":            "/tmp",
	} {
		if got := tildify(in); got != want {
			t.Errorf("tildify(%q) = %q, want %q", in, got, want)
		}
	}
}
