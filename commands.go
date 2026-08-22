package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/dan2756/dirnote/internal/notes"
)

const addHelp = `Usage: dirnote add [-d DIR] [--local] TEXT...

Append TEXT to the note for the current directory, on a new line. The words
are joined with spaces, so quoting is optional. With no TEXT the note is read
from standard input.

  -d DIR    act on DIR instead of the current directory
  --local   write to DIR/.dirnote instead of the central store
`

const setHelp = `Usage: dirnote set [-d DIR] [--local] TEXT...

Replace the note for the current directory with TEXT. With no TEXT the note
is read from standard input.

  -d DIR    act on DIR instead of the current directory
  --local   write to DIR/.dirnote instead of the central store
`

const editHelp = `Usage: dirnote edit [-d DIR] [--local]

Open the note for the current directory in $VISUAL, or $EDITOR, or vi.
Saving an empty file deletes the note.

  -d DIR    act on DIR instead of the current directory
  --local   edit DIR/.dirnote instead of the note in the central store
`

const rmHelp = `Usage: dirnote rm [-d DIR] [--local]

Delete the note for the current directory. DIR does not have to exist any
more. Exits 1 if there was no note.

  -d DIR    act on DIR instead of the current directory
  --local   delete DIR/.dirnote instead of the note in the central store
`

func (c *cli) add(args []string) error { return c.write("add", args, false) }
func (c *cli) set(args []string) error { return c.write("set", args, true) }

func (c *cli) write(name string, args []string, replace bool) error {
	flags := newFlags(name)
	dirFlag := flags.String("d", "", "")
	local := flags.Bool("local", false, "")
	pos, err := parse(flags, args)
	if err != nil {
		return err
	}
	dir, err := targetDir(*dirFlag, true)
	if err != nil {
		return err
	}
	text, err := c.noteText(pos)
	if err != nil {
		return err
	}
	ref, err := openNote(dir, *local)
	if err != nil {
		return err
	}
	defer ref.close()
	if !replace {
		old, err := ref.get()
		if err != nil {
			return err
		}
		if old != "" {
			text = old + "\n" + text
		}
	}
	if err := ref.put(text, c.now()); err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "saved %s\n", ref)
	return nil
}

// noteText joins the positional arguments, or reads stdin when there are
// none and stdin is not a terminal.
func (c *cli) noteText(pos []string) (string, error) {
	var text string
	if len(pos) > 0 {
		text = strings.Join(pos, " ")
	} else if !isTerminal(c.stdin) {
		b, err := io.ReadAll(c.stdin)
		if err != nil {
			return "", err
		}
		text = string(b)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errUsage{"no note text given"}
	}
	return text, nil
}

func (c *cli) edit(args []string) error {
	flags := newFlags("edit")
	dirFlag := flags.String("d", "", "")
	local := flags.Bool("local", false, "")
	if err := noArgs(flags, args); err != nil {
		return err
	}
	dir, err := targetDir(*dirFlag, true)
	if err != nil {
		return err
	}
	ref, err := openNote(dir, *local)
	if err != nil {
		return err
	}
	old, err := ref.get()
	ref.close() // not held while the editor is open
	if err != nil {
		return err
	}
	text, err := c.runEditor(old)
	if err != nil {
		return err
	}
	if text == old {
		fmt.Fprintln(c.stdout, "no changes")
		return nil
	}

	// Other notes may have been saved while the editor was open: apply the
	// change to the store as it is now, not to the copy read before.
	ref, err = openNote(dir, *local)
	if err != nil {
		return err
	}
	defer ref.close()
	if text == "" {
		if _, err := ref.remove(); err != nil {
			return err
		}
		fmt.Fprintf(c.stdout, "removed %s\n", ref)
		return nil
	}
	if err := ref.put(text, c.now()); err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "saved %s\n", ref)
	return nil
}

// runEditor opens text in the user's editor and returns what they saved,
// trimmed.
func (c *cli) runEditor(text string) (string, error) {
	f, err := os.CreateTemp("", "dirnote-*.txt")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if text != "" {
		text += "\n"
	}
	if _, err := f.WriteString(text); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}

	editor := editorCommand()
	// Through sh so that EDITOR="code --wait" and friends work.
	cmd := exec.Command("sh", "-c", editor+` "$1"`, "sh", f.Name())
	cmd.Stdin, cmd.Stdout, cmd.Stderr = c.stdin, c.stdout, c.stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor %q: %w", editor, err)
	}
	b, err := os.ReadFile(f.Name())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func editorCommand() string {
	for _, v := range []string{"VISUAL", "EDITOR"} {
		if e := strings.TrimSpace(os.Getenv(v)); e != "" {
			return e
		}
	}
	return "vi"
}

func (c *cli) rm(args []string) error {
	flags := newFlags("rm")
	dirFlag := flags.String("d", "", "")
	local := flags.Bool("local", false, "")
	if err := noArgs(flags, args); err != nil {
		return err
	}
	dir, err := targetDir(*dirFlag, false)
	if err != nil {
		return err
	}
	ref, err := openNote(dir, *local)
	if err != nil {
		return err
	}
	defer ref.close()
	removed, err := ref.remove()
	if err != nil {
		return err
	}
	if !removed {
		return errNoNote{dir: dir, local: *local}
	}
	fmt.Fprintf(c.stdout, "removed %s\n", ref)
	return nil
}

// noteRef is the note that add, set, edit and rm change: dir's entry in the
// central store, or with --local its .dirnote file.
type noteRef struct {
	dir string
	st  *notes.Store // nil for a .dirnote file
}

// openNote opens dir's note for changing. The store stays locked against
// other dirnote processes until close, so that notes they save meanwhile are
// not overwritten.
func openNote(dir string, local bool) (*noteRef, error) {
	if local {
		return &noteRef{dir: dir}, nil
	}
	st, err := loadStore(true)
	if err != nil {
		return nil, err
	}
	return &noteRef{dir: dir, st: st}, nil
}

func (r *noteRef) close() {
	if r.st != nil {
		r.st.Unlock()
	}
}

// String names the note in messages.
func (r *noteRef) String() string {
	if r.st == nil {
		return tildify(notes.LocalPath(r.dir))
	}
	return "note for " + tildify(r.dir)
}

func (r *noteRef) get() (string, error) {
	if r.st == nil {
		// A symlinked .dirnote is replaced, not written through, and what
		// it points to (in a cloned repository, any file at all) must not
		// be copied into the new one.
		if fi, err := os.Lstat(notes.LocalPath(r.dir)); err == nil && fi.Mode()&fs.ModeSymlink != 0 {
			return "", nil
		}
		text, _, err := notes.ReadLocal(r.dir)
		return text, err
	}
	if n, ok := r.st.Get(r.dir); ok {
		return n.Text, nil
	}
	return "", nil
}

func (r *noteRef) put(text string, now time.Time) error {
	if r.st == nil {
		return notes.WriteLocal(r.dir, text)
	}
	r.st.Set(r.dir, text, now)
	return r.st.Save()
}

// remove deletes the note and reports whether there was one.
func (r *noteRef) remove() (bool, error) {
	if r.st == nil {
		return notes.RemoveLocal(r.dir)
	}
	if !r.st.Remove(r.dir) {
		return false, nil
	}
	return true, r.st.Save()
}

// loadStore reads the central store. With lock it stays locked against
// other dirnote processes until Unlock, which commands that change it need.
func loadStore(lock bool) (*notes.Store, error) {
	path, err := notes.DefaultPath()
	if err != nil {
		return nil, err
	}
	if lock {
		return notes.LoadLocked(path)
	}
	return notes.Load(path)
}

// targetDir resolves the -d flag, or the working directory when it is empty.
// With mustExist the result has to be an existing directory.
func targetDir(flagVal string, mustExist bool) (string, error) {
	p := flagVal
	if p == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		p = wd
	}
	dir, err := notes.Resolve(p)
	if err != nil {
		return "", err
	}
	if mustExist {
		fi, err := os.Stat(dir)
		if err != nil {
			return "", err
		}
		if !fi.IsDir() {
			return "", fmt.Errorf("%s is not a directory", p)
		}
	}
	return dir, nil
}

// dirExists reports whether dir is still there. Errors other than "not
// found" (permissions, an unreadable mount) count as existing so prune
// never drops a note it isn't sure about.
func dirExists(dir string) bool {
	_, err := os.Stat(dir)
	return !errors.Is(err, fs.ErrNotExist)
}
