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

// noteOf returns the note text for dir as printed by show --plain, or "" if
// there is none.
func noteOf(t *testing.T, dir string) string {
	t.Helper()
	r := runCLI(t, "", "show", "--plain", "-d", dir)
	if r.code != 0 {
		return ""
	}
	header, body, _ := strings.Cut(r.stdout, "\n")
	if header != tildify(dir)+":" {
		t.Fatalf("unexpected header %q", header)
	}
	var ls []string
	for _, l := range strings.Split(strings.TrimSuffix(body, "\n"), "\n") {
		ls = append(ls, strings.TrimPrefix(l, "  "))
	}
	return strings.Join(ls, "\n")
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
	if got := noteOf(t, dir); got != "run make dev first\nneeds docker" {
		t.Fatalf("note = %q", got)
	}

	if r := runCLI(t, "", "set", "-d", dir, "just one line"); r.code != 0 {
		t.Fatalf("set: %+v", r)
	}
	if r := runCLI(t, "", "-d", dir); !strings.Contains(r.stdout, "│ just one line ") {
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
	if got := noteOf(t, dir); got != "line one\nline two" {
		t.Fatalf("note = %q", got)
	}
	if r := runCLI(t, "  \n", "add", "-d", dir); r.code != exitError {
		t.Fatalf("empty add should fail: %+v", r)
	}
}

func TestDashDash(t *testing.T) {
	dir := setup(t)
	runCLI(t, "", "set", "-d", dir, "--", "-v", "is", "verbose")
	if got := noteOf(t, dir); got != "-v is verbose" {
		t.Fatalf("note = %q", got)
	}
}

func TestDashedText(t *testing.T) {
	dir := setup(t)
	runCLI(t, "", "add", "-d", dir, "- seed the db first")
	runCLI(t, "", "add", "- then run make dev", "-d", dir)
	if got := noteOf(t, dir); got != "- seed the db first\n- then run make dev" {
		t.Fatalf("note = %q", got)
	}
}

func TestSymlinkedDirSharesNote(t *testing.T) {
	dir := setup(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	runCLI(t, "", "add", "-d", link, "via the link")
	if got := noteOf(t, dir); got != "via the link" {
		t.Fatalf("note = %q", got)
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

func TestShowHook(t *testing.T) {
	dir := setup(t)
	if r := runCLI(t, "", "show", "--hook", "-d", dir); r != (result{}) {
		t.Fatalf("hook with no note should be silent: %+v", r)
	}
	runCLI(t, "", "add", "-d", dir, "needs docker")
	r := runCLI(t, "", "show", "--hook", "-d", dir)
	if r.code != 0 || r.stdout != "" || !strings.Contains(r.stderr, "│ needs docker ") {
		t.Fatalf("hook should print the note to stderr: %+v", r)
	}

	os.WriteFile(os.Getenv("DIRNOTE_FILE"), []byte("{broken"), 0o600)
	if r := runCLI(t, "", "show", "--hook", "-d", dir); r.code != 0 || r.stderr == "" {
		t.Fatalf("hook should report errors but exit 0: %+v", r)
	}
}

func TestInit(t *testing.T) {
	for _, sh := range []string{"zsh", "bash", "fish"} {
		r := runCLI(t, "", "init", sh)
		if r.code != 0 || !strings.Contains(r.stdout, "dirnote show --hook") {
			t.Errorf("init %s: %+v", sh, r)
		}
	}
	if r := runCLI(t, "", "init", "tcsh"); r.code != exitError {
		t.Errorf("init tcsh: %+v", r)
	}
	if r := runCLI(t, "", "init"); r.code != exitError {
		t.Errorf("init with no shell: %+v", r)
	}
}
