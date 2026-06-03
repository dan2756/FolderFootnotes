package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
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

const showHelp = `Usage: dirnote show [-d DIR]

Print the note for the current directory. Exits 1 if there is none.

  -d DIR   act on DIR instead of the current directory
`

const rmHelp = `Usage: dirnote rm [-d DIR]

Delete the note for the current directory. DIR does not have to exist any
more. Exits 1 if there was no note.

  -d DIR   act on DIR instead of the current directory
`

const lsHelp = `Usage: dirnote ls

List every note in the store with its directory. Directories that no longer
exist are flagged as missing.
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

func (c *cli) show(args []string) error {
	flags := newFlags("show")
	dirFlag := flags.String("d", "", "")
	if pos, err := parse(flags, args); err != nil {
		return err
	} else if len(pos) > 0 {
		return errUsage{"unexpected argument " + pos[0]}
	}
	dir, err := targetDir(*dirFlag, false)
	if err != nil {
		return err
	}
	st, err := loadStore()
	if err != nil {
		return err
	}
	n, ok := st.Get(dir)
	if !ok {
		return errNoNote{dir}
	}
	fmt.Fprintln(c.stdout, n.Text)
	return nil
}

func (c *cli) rm(args []string) error {
	flags := newFlags("rm")
	dirFlag := flags.String("d", "", "")
	if pos, err := parse(flags, args); err != nil {
		return err
	} else if len(pos) > 0 {
		return errUsage{"unexpected argument " + pos[0]}
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

func (c *cli) ls(args []string) error {
	flags := newFlags("ls")
	if pos, err := parse(flags, args); err != nil {
		return err
	} else if len(pos) > 0 {
		return errUsage{"unexpected argument " + pos[0]}
	}
	st, err := loadStore()
	if err != nil {
		return err
	}
	if len(st.Notes) == 0 {
		fmt.Fprintln(c.stderr, "no notes yet; add one with 'dirnote add TEXT'")
		return nil
	}
	for _, dir := range st.Dirs() {
		line := tildify(dir)
		if !dirExists(dir) {
			line += " (missing)"
		}
		fmt.Fprintln(c.stdout, line)
		for _, l := range strings.Split(st.Notes[dir].Text, "\n") {
			fmt.Fprintln(c.stdout, "  "+l)
		}
	}
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
// found" (permissions, say) count as existing.
func dirExists(dir string) bool {
	_, err := os.Stat(dir)
	return !errors.Is(err, fs.ErrNotExist)
}

func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
