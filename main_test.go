package main

import (
	"bytes"
	"encoding/json"
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

func TestAddRejectsInvalidUTF8(t *testing.T) {
	dir := setup(t)
	r := runCLI(t, "caf\xe9\n", "add", "-d", dir)
	if r.code != exitError || !strings.Contains(r.stderr, "UTF-8") {
		t.Fatalf("add: %+v", r)
	}
	if r := runCLI(t, "", "show", "-d", dir); r.code != exitNoNote {
		t.Fatalf("invalid text was stored: %+v", r)
	}
	// A .dirnote file is raw bytes, not JSON, so it can keep them.
	if r := runCLI(t, "caf\xe9\n", "add", "--local", "-d", dir); r.code != 0 {
		t.Fatalf("add --local: %+v", r)
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

// fakeEditor makes $VISUAL a script that runs body with the file in $1.
func fakeEditor(t *testing.T, body string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "editor")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", path)
}

func TestEdit(t *testing.T) {
	dir := setup(t)
	runCLI(t, "", "add", "-d", dir, "first")

	fakeEditor(t, `echo second >> "$1"`)
	if r := runCLI(t, "", "edit", "-d", dir); r.code != 0 {
		t.Fatalf("edit: %+v", r)
	}
	if got := noteOf(t, dir); got != "first\nsecond" {
		t.Fatalf("note = %q", got)
	}

	fakeEditor(t, `true`)
	if r := runCLI(t, "", "edit", "-d", dir); r.code != 0 || r.stdout != "no changes\n" {
		t.Fatalf("unchanged edit: %+v", r)
	}

	fakeEditor(t, `: > "$1"`)
	runCLI(t, "", "edit", "-d", dir)
	if r := runCLI(t, "", "show", "-d", dir); r.code != exitNoNote {
		t.Fatalf("emptied note should be deleted: %+v", r)
	}

	fakeEditor(t, `exit 3`)
	if r := runCLI(t, "", "edit", "-d", dir); r.code != exitError {
		t.Fatalf("failing editor: %+v", r)
	}
}

func TestPrune(t *testing.T) {
	dir := setup(t)
	gone := filepath.Join(dir, "gone")
	os.Mkdir(gone, 0o755)
	runCLI(t, "", "add", "-d", dir, "kept")
	runCLI(t, "", "add", "-d", gone, "stale")
	os.Remove(gone)

	r := runCLI(t, "", "prune", "--dry-run")
	if r.stdout != "would delete note for "+tildify(gone)+"\n" {
		t.Fatalf("dry run: %+v", r)
	}
	if r := runCLI(t, "", "show", "-d", gone); r.code != 0 {
		t.Fatalf("dry run should keep the note: %+v", r)
	}
	if r := runCLI(t, "", "prune"); r.stdout != "deleted note for "+tildify(gone)+"\n" {
		t.Fatalf("prune: %+v", r)
	}
	if r := runCLI(t, "", "show", "-d", gone); r.code != exitNoNote {
		t.Fatalf("pruned note still there: %+v", r)
	}
	if r := runCLI(t, "", "prune"); r.code != 0 || r.stdout != "" {
		t.Fatalf("second prune: %+v", r)
	}
}

func TestJSON(t *testing.T) {
	dir := setup(t)
	gone := filepath.Join(dir, "gone")
	os.Mkdir(gone, 0o755)
	runCLI(t, "", "add", "-d", dir, "hello")
	runCLI(t, "", "add", "-d", gone, "bye")
	os.Remove(gone)

	r := runCLI(t, "", "show", "--json", "-d", dir)
	var shown []noteJSON
	if err := json.Unmarshal([]byte(r.stdout), &shown); err != nil || r.code != 0 {
		t.Fatalf("show --json: %v %+v", err, r)
	}
	if len(shown) != 1 || shown[0].Dir != dir || shown[0].Text != "hello" || shown[0].Source != "store" || shown[0].Updated.IsZero() {
		t.Fatalf("show --json = %+v", shown)
	}

	r = runCLI(t, "", "show", "--json", "-d", filepath.Join(dir, "nothing"))
	if r.code != exitNoNote || strings.TrimSpace(r.stdout) != "[]" {
		t.Fatalf("show --json without note: %+v", r)
	}

	r = runCLI(t, "", "ls", "--json")
	var listed []lsJSON
	if err := json.Unmarshal([]byte(r.stdout), &listed); err != nil {
		t.Fatalf("ls --json: %v %+v", err, r)
	}
	if len(listed) != 2 || listed[0].Dir != dir || listed[0].Missing || listed[1].Dir != gone || !listed[1].Missing {
		t.Fatalf("ls --json = %+v", listed)
	}
}

func TestLocalNotes(t *testing.T) {
	dir := setup(t)
	if r := runCLI(t, "", "add", "--local", "-d", dir, "use node 20"); r.code != 0 {
		t.Fatalf("add --local: %+v", r)
	}
	runCLI(t, "", "add", "--local", "-d", dir, "make seed before tests")
	b, err := os.ReadFile(filepath.Join(dir, ".dirnote"))
	if err != nil || string(b) != "use node 20\nmake seed before tests\n" {
		t.Fatalf(".dirnote = %q, %v", b, err)
	}
	if r := runCLI(t, "", "ls"); r.stdout != "" {
		t.Fatalf("local notes should stay out of the store: %+v", r)
	}

	runCLI(t, "", "add", "-d", dir, "personal reminder")
	r := runCLI(t, "", "show", "--plain", "-d", dir)
	want := tildify(dir) + ":\n  personal reminder\n" +
		tildify(filepath.Join(dir, ".dirnote")) + ":\n  use node 20\n  make seed before tests\n"
	if r.code != 0 || r.stdout != want {
		t.Fatalf("show both:\n%s\nwant:\n%s", r.stdout, want)
	}

	runCLI(t, "", "set", "--local", "-d", dir, "replaced")
	fakeEditor(t, `echo edited >> "$1"`)
	if r := runCLI(t, "", "edit", "--local", "-d", dir); r.code != 0 {
		t.Fatalf("edit --local: %+v", r)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, ".dirnote")); string(b) != "replaced\nedited\n" {
		t.Fatalf(".dirnote after edit = %q", b)
	}

	if r := runCLI(t, "", "rm", "--local", "-d", dir); r.code != 0 {
		t.Fatalf("rm --local: %+v", r)
	}
	if r := runCLI(t, "", "rm", "--local", "-d", dir); r.code != exitNoNote {
		t.Fatalf("second rm --local: %+v", r)
	}
	if got := noteOf(t, dir); got != "personal reminder" {
		t.Fatalf("store note should survive rm --local: %q", got)
	}
}

func TestHookShowsOnlyLocal(t *testing.T) {
	dir := setup(t)
	os.WriteFile(filepath.Join(dir, ".dirnote"), []byte("from the repo \x1b]0;gotcha\x07\n"), 0o644)
	r := runCLI(t, "", "show", "--hook", "--plain", "-d", dir)
	want := tildify(filepath.Join(dir, ".dirnote")) + ":\n  from the repo\n"
	if r.code != 0 || r.stderr != want {
		t.Fatalf("hook: %+v", r)
	}
}

func TestInheritHook(t *testing.T) {
	root := setup(t)
	t.Setenv("DIRNOTE_INHERIT", "")
	proj := filepath.Join(root, "proj")
	pkg := filepath.Join(proj, "src", "pkg")
	other := filepath.Join(root, "other")
	for _, d := range []string{pkg, other} {
		os.MkdirAll(d, 0o755)
	}
	runCLI(t, "", "add", "-d", proj, "run make dev first")

	hook := func(args ...string) string {
		t.Helper()
		r := runCLI(t, "", append([]string{"show", "--hook", "--plain"}, args...)...)
		if r.code != 0 || r.stdout != "" {
			t.Fatalf("hook %v: %+v", args, r)
		}
		return r.stderr
	}
	projNote := tildify(proj) + ":\n  run make dev first\n"

	if got := hook("-d", pkg, "--from", other); got != "" {
		t.Errorf("without --inherit a subdirectory shows nothing, got %q", got)
	}
	if got := hook("-d", pkg, "--from", other, "--inherit"); got != projNote {
		t.Errorf("entering from outside: %q", got)
	}
	if got := hook("-d", pkg, "--from", proj, "--inherit"); got != "" {
		t.Errorf("moving inside the project: %q", got)
	}
	if got := hook("-d", proj, "--from", proj, "--inherit"); got != "" {
		t.Errorf("same directory: %q", got)
	}

	t.Setenv("DIRNOTE_INHERIT", "1")
	if got := hook("-d", pkg, "--from", other); got != projNote {
		t.Errorf("DIRNOTE_INHERIT=1: %q", got)
	}
	if got := hook("-d", pkg, "--from", other, "--inherit=false"); got != "" {
		t.Errorf("--inherit=false should override the env: %q", got)
	}

	r := runCLI(t, "", "show", "--json", "-d", pkg)
	var shown []noteJSON
	if err := json.Unmarshal([]byte(r.stdout), &shown); err != nil || len(shown) != 1 || !shown[0].Inherited || shown[0].Dir != proj {
		t.Errorf("show --json with inherit: %v %+v", err, r)
	}

	if r := runCLI(t, "", "show", "--from", other); r.code != exitError {
		t.Errorf("--from without --hook should be rejected: %+v", r)
	}
}

func TestInitFlags(t *testing.T) {
	r := runCLI(t, "", "init", "zsh", "--inherit")
	if r.code != 0 || !strings.Contains(r.stdout, "show --hook --inherit --from") {
		t.Fatalf("init zsh --inherit: %+v", r)
	}
	if r := runCLI(t, "", "init", "bash", "--bogus"); r.code != exitError {
		t.Fatalf("unknown init flag: %+v", r)
	}
}

func TestEditKeepsTextWhenSaveFails(t *testing.T) {
	dir := setup(t)
	store := os.Getenv("DIRNOTE_FILE")
	t.Cleanup(func() { os.Chmod(filepath.Dir(store), 0o755) })
	if r := runCLI(t, "", "add", "-d", dir, "original"); r.code != 0 {
		t.Fatalf("add: %+v", r)
	}

	// The store can be read but not replaced: the directory is not writable.
	fakeEditor(t, `echo edited > "$1"; chmod 555 "$(dirname "$DIRNOTE_FILE")"`)
	r := runCLI(t, "", "edit", "-d", dir)
	if r.code != exitError {
		t.Fatalf("edit: %+v", r)
	}
	const marker = "edited text kept in "
	i := strings.LastIndex(r.stderr, marker)
	if i < 0 {
		t.Fatalf("stderr: %s", r.stderr)
	}
	path := strings.TrimSpace(r.stderr[i+len(marker):])
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "edited\n" {
		t.Fatalf("rescue file %q: %q, %v", path, b, err)
	}
	os.Chmod(filepath.Dir(store), 0o755)
	if got := noteOf(t, dir); got != "original" {
		t.Fatalf("note changed despite the failed save: %q", got)
	}
}

func TestEditKeepsNotesSavedMeanwhile(t *testing.T) {
	dir := setup(t)
	runCLI(t, "", "add", "-d", dir, "original")
	// Another dirnote process saves a note while the editor is open.
	other := `{"version":1,"notes":{"/elsewhere":{"text":"saved meanwhile","created":"2026-06-03T12:00:00Z","updated":"2026-06-03T12:00:00Z"}}}`
	fakeEditor(t, `printf '%s\n' '`+other+`' > "$DIRNOTE_FILE"; echo edited > "$1"`)
	if r := runCLI(t, "", "edit", "-d", dir); r.code != 0 {
		t.Fatalf("edit: %+v", r)
	}
	if got := noteOf(t, dir); got != "edited" {
		t.Fatalf("note = %q", got)
	}
	r := runCLI(t, "", "show", "--plain", "-d", "/elsewhere")
	if r.code != 0 || !strings.Contains(r.stdout, "saved meanwhile") {
		t.Fatalf("note saved during the edit was lost: %+v", r)
	}
}

func TestShowDoesNotReadSymlinkedDirnote(t *testing.T) {
	dir := setup(t)
	outside := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(outside, []byte("PRIVATE KEY\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(dir, outside)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(rel, filepath.Join(dir, ".dirnote")); err != nil {
		t.Fatal(err)
	}
	if r := runCLI(t, "", "add", "-d", dir, "personal reminder"); r.code != 0 {
		t.Fatalf("add: %+v", r)
	}

	r := runCLI(t, "", "show", "--plain", "-d", dir)
	blob := r.stdout + r.stderr
	if strings.Contains(blob, "PRIVATE KEY") {
		t.Fatalf("printed the symlink target: %+v", r)
	}
	if r.code != exitError || !strings.Contains(r.stderr, "symlink") || !strings.Contains(r.stdout, "personal reminder") {
		t.Fatalf("show: %+v", r)
	}

	r = runCLI(t, "", "show", "--hook", "--plain", "-d", dir)
	if r.code != 0 || r.stdout != "" || strings.Contains(r.stderr, "PRIVATE KEY") {
		t.Fatalf("hook leaked or failed: %+v", r)
	}
	if !strings.Contains(r.stderr, "symlink") || !strings.Contains(r.stderr, "personal reminder") {
		t.Fatalf("hook: %+v", r)
	}
}

func TestLocalSymlinkTargetNotCopied(t *testing.T) {
	dir := setup(t)
	secret := filepath.Join(t.TempDir(), "id_ed25519")
	os.WriteFile(secret, []byte("PRIVATE KEY\n"), 0o600)
	link := filepath.Join(dir, ".dirnote")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}
	if r := runCLI(t, "", "add", "--local", "-d", dir, "use node 20"); r.code != 0 {
		t.Fatalf("add --local: %+v", r)
	}
	if b, _ := os.ReadFile(link); string(b) != "use node 20\n" {
		t.Fatalf(".dirnote = %q, want only the new text", b)
	}
	if b, _ := os.ReadFile(secret); string(b) != "PRIVATE KEY\n" {
		t.Fatalf("link target changed: %q", b)
	}

	os.Remove(link)
	os.Symlink(secret, link)
	fakeEditor(t, `[ -s "$1" ] && exit 1; echo edited > "$1"`)
	if r := runCLI(t, "", "edit", "--local", "-d", dir); r.code != 0 {
		t.Fatalf("edit --local should start empty: %+v", r)
	}
}

func TestMessagesStripEscapes(t *testing.T) {
	root := setup(t)
	dir := filepath.Join(root, "docs\x1b]52;c;cHduZWQ=\x07")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Skip("file system refuses the name:", err)
	}
	check := func(what string, r result) {
		t.Helper()
		if strings.ContainsAny(r.stdout+r.stderr, "\x1b\x07") {
			t.Errorf("%s printed an escape sequence: %+v", what, r)
		}
	}
	check("show", runCLI(t, "", "show", "-d", dir))
	check("add", runCLI(t, "", "add", "-d", dir, "hi"))
	check("ls", runCLI(t, "", "ls"))
	check("rm", runCLI(t, "", "rm", "-d", dir))
	os.Symlink(".dirnote", filepath.Join(dir, ".dirnote")) // a loop: unreadable
	r := runCLI(t, "", "show", "--hook", "--from", root, "-d", dir)
	if r.stderr == "" {
		t.Fatalf("hook should report the unreadable .dirnote: %+v", r)
	}
	check("hook", r)
}

func TestShowJSONStripsLocalText(t *testing.T) {
	dir := setup(t)
	os.WriteFile(filepath.Join(dir, ".dirnote"), []byte("hello \x1b]0;PWNED\x07 ‮evil\n"), 0o644)
	r := runCLI(t, "", "show", "--json", "-d", dir)
	var shown []noteJSON
	if err := json.Unmarshal([]byte(r.stdout), &shown); err != nil || len(shown) != 1 {
		t.Fatalf("show --json: %v %+v", err, r)
	}
	if shown[0].Text != "hello  evil" {
		t.Fatalf("text = %q", shown[0].Text)
	}
}
