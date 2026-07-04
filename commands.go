package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	"github.com/dan2756/dirnote/internal/notes"
)

const addHelp = `Usage: dirnote add [-d DIR] TEXT...

Append TEXT to the note for the current directory, on a new line. The words
are joined with spaces, so quoting is optional. With no TEXT the note is read
from standard input.

  -d DIR   act on DIR instead of the current directory
`

const setHelp = `Usage: dirnote set [-d DIR] TEXT...

Replace the note for the current directory with TEXT. With no TEXT the note
is read from standard input.

  -d DIR   act on DIR instead of the current directory
`

const editHelp = `Usage: dirnote edit [-d DIR]

Open the note for the current directory in $VISUAL, or $EDITOR, or vi.
Saving an empty file deletes the note.

  -d DIR   act on DIR instead of the current directory
`

const rmHelp = `Usage: dirnote rm [-d DIR]

Delete the note for the current directory. DIR does not have to exist any
more. Exits 1 if there was no note.

  -d DIR   act on DIR instead of the current directory
`

func (c *cli) add(args []string) error { return c.write("add", args, false) }
func (c *cli) set(args []string) error { return c.write("set", args, true) }

func (c *cli) write(name string, args []string, replace bool) error {
	flags := newFlags(name)
	dirFlag := flags.String("d", "", "")
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

	st, err := loadStore()
	if err != nil {
		return err
	}
	if replace {
		st.Set(dir, text, c.now())
	} else {
		st.Append(dir, text, c.now())
	}
	if err := st.Save(); err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "saved note for %s\n", tildify(dir))
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
	if err := noArgs(flags, args); err != nil {
		return err
	}
	dir, err := targetDir(*dirFlag, true)
	if err != nil {
		return err
	}
	st, err := loadStore()
	if err != nil {
		return err
	}
	var old string
	if n, ok := st.Get(dir); ok {
		old = n.Text
	}
	text, err := c.runEditor(old)
	if err != nil {
		return err
	}

	switch {
	case text == old:
		fmt.Fprintln(c.stdout, "no changes")
		return nil
	case text == "":
		st.Remove(dir)
	default:
		st.Set(dir, text, c.now())
	}
	if err := st.Save(); err != nil {
		return err
	}
	if text == "" {
		fmt.Fprintf(c.stdout, "removed note for %s\n", tildify(dir))
	} else {
		fmt.Fprintf(c.stdout, "saved note for %s\n", tildify(dir))
	}
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
	if err := noArgs(flags, args); err != nil {
		return err
	}
	dir, err := targetDir(*dirFlag, false)
	if err != nil {
		return err
	}
	st, err := loadStore()
	if err != nil {
		return err
	}
	if !st.Remove(dir) {
		return errNoNote{dir}
	}
	if err := st.Save(); err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "removed note for %s\n", tildify(dir))
	return nil
}

func loadStore() (*notes.Store, error) {
	path, err := notes.DefaultPath()
	if err != nil {
		return nil, err
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
