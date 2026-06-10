// Command dirnote attaches short notes to directories and shows them when
// you cd in.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"
)

// version is set at build time with -ldflags "-X main.version=v1.2.3".
var version string

const usage = `dirnote - leave a note on a directory, see it when you cd in

Usage:
  dirnote add [-d DIR] TEXT...    append to the note for a directory
  dirnote set [-d DIR] TEXT...    replace the note
  dirnote show [-d DIR] [--plain] print the note (the default command)
  dirnote rm [-d DIR]             delete the note
  dirnote ls                      list every note
  dirnote init zsh|bash|fish      print the shell hook that shows notes on cd

Commands act on the current directory unless -d is given. Notes are kept in
$XDG_DATA_HOME/dirnote/notes.json (~/.local/share/dirnote/notes.json), or in
$DIRNOTE_FILE if set.

To see notes as you cd around, add one of these to your shell's rc file:
  eval "$(dirnote init zsh)"     # ~/.zshrc
  eval "$(dirnote init bash)"    # ~/.bashrc
  dirnote init fish | source     # ~/.config/fish/config.fish

Run 'dirnote COMMAND -h' for details on a command.
`

type command struct {
	name string
	help string
	run  func(c *cli, args []string) error
}

var commands []command

func init() {
	commands = []command{
		{"add", addHelp, (*cli).add},
		{"set", setHelp, (*cli).set},
		{"show", showHelp, (*cli).show},
		{"rm", rmHelp, (*cli).rm},
		{"ls", lsHelp, (*cli).ls},
		{"init", initHelp, (*cli).initShell},
	}
}

// cli holds what commands need from the outside world, so tests can swap it.
type cli struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	now    func() time.Time
}

func main() {
	c := &cli{stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr, now: time.Now}
	os.Exit(c.run(os.Args[1:]))
}

// Exit codes.
const (
	exitOK     = 0
	exitNoNote = 1 // show or rm found nothing
	exitError  = 2 // bad usage or a real failure
)

// errNoNote reports that a directory has no note.
type errNoNote struct{ dir string }

func (e errNoNote) Error() string { return "no note for " + tildify(e.dir) }

// errUsage is a mistake on the command line.
type errUsage struct{ msg string }

func (e errUsage) Error() string { return e.msg }

func (c *cli) run(args []string) int {
	name := "show"
	if len(args) > 0 {
		switch args[0] {
		case "-h", "-help", "--help", "help":
			if len(args) > 1 {
				if cmd, ok := lookup(args[1]); ok {
					fmt.Fprint(c.stdout, cmd.help)
					return exitOK
				}
			}
			fmt.Fprint(c.stdout, usage)
			return exitOK
		case "-version", "--version", "version":
			fmt.Fprintln(c.stdout, "dirnote", versionString())
			return exitOK
		}
		if !strings.HasPrefix(args[0], "-") {
			name, args = args[0], args[1:]
		}
	}
	cmd, ok := lookup(name)
	if !ok {
		fmt.Fprintf(c.stderr, "dirnote: unknown command %q\nRun 'dirnote -h' for usage.\n", name)
		return exitError
	}

	err := cmd.run(c, args)
	var noNote errNoNote
	var bad errUsage
	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, flag.ErrHelp):
		fmt.Fprint(c.stdout, cmd.help)
		return exitOK
	case errors.As(err, &noNote):
		fmt.Fprintf(c.stderr, "dirnote: %v\n", err)
		return exitNoNote
	case errors.As(err, &bad):
		fmt.Fprintf(c.stderr, "dirnote %s: %v\nRun 'dirnote %s -h' for usage.\n", cmd.name, err, cmd.name)
		return exitError
	default:
		fmt.Fprintf(c.stderr, "dirnote: %v\n", err)
		return exitError
	}
}

func lookup(name string) (command, bool) {
	for _, cmd := range commands {
		if cmd.name == name {
			return cmd, true
		}
	}
	return command{}, false
}

func versionString() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}

// newFlags returns a flag set whose errors are reported by run.
func newFlags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// parse parses flags wherever they appear among args (the flag package stops
// at the first positional argument) and returns the positional arguments.
// "--" ends flag parsing, and a dash followed by whitespace ("- seed the db")
// is note text, not a flag.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos, flagArgs []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		name, hasValue, ok := flagName(a)
		if !ok {
			pos = append(pos, a)
			continue
		}
		flagArgs = append(flagArgs, a)
		if !hasValue && takesValue(fs, name) && i+1 < len(args) {
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, err
		}
		return nil, errUsage{err.Error()}
	}
	return append(pos, fs.Args()...), nil
}

// flagName splits "-name" or "--name=value" into the name and whether a
// value is attached. ok is false for anything that is not a flag.
func flagName(a string) (name string, hasValue, ok bool) {
	if len(a) < 2 || a[0] != '-' {
		return "", false, false
	}
	name = strings.TrimPrefix(a[1:], "-")
	name, _, hasValue = strings.Cut(name, "=")
	if name == "" || strings.ContainsAny(name, " \t\n") {
		return "", false, false
	}
	return name, hasValue, true
}

func takesValue(fs *flag.FlagSet, name string) bool {
	f := fs.Lookup(name)
	if f == nil {
		return false
	}
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return !ok || !b.IsBoolFlag()
}

// tildify shortens paths under the home directory to ~/...
func tildify(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	for _, h := range []string{home, resolved(home)} {
		if path == h {
			return "~"
		}
		if rest, ok := strings.CutPrefix(path, h+string(filepath.Separator)); ok && h != "/" {
			return "~" + string(filepath.Separator) + rest
		}
	}
	return path
}

func resolved(path string) string {
	if r, err := filepath.EvalSymlinks(path); err == nil {
		return r
	}
	return path
}
