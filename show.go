package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dan2756/dirnote/internal/notes"
	"github.com/dan2756/dirnote/internal/render"
	"github.com/dan2756/dirnote/internal/shell"
)

const showHelp = `Usage: dirnote show [-d DIR] [--inherit] [--plain | --json]
       dirnote show --hook [--from DIR] [--inherit] [--plain]

Print the notes for the current directory in a box sized to the terminal:
the one in the central store and the directory's .dirnote file, if either
exists. Exits 1 if there is none.

  -d DIR       act on DIR instead of the current directory
  --inherit    also show the notes of parent directories, closest first
               (default from DIRNOTE_INHERIT=1)
  --plain      no box and no colors, just the directory and the note
  --json       print a JSON array of notes (empty when there is none)
  --hook       mode for shell hooks: print to stderr, say nothing and exit 0
               when there is no note
  --from DIR   with --hook, the directory the shell was in before. Nothing
               is shown if it is the current one, and a parent's note only
               when entering its subtree from outside.

Colors are left out when the output is not a terminal or NO_COLOR is set.
`

const initHelp = `Usage: dirnote init zsh|bash|fish [--inherit] [--plain]

Print a hook that runs 'dirnote show --hook' whenever the working directory
changes. Load it from your shell's startup file:

  eval "$(dirnote init zsh)"     # ~/.zshrc
  eval "$(dirnote init bash)"    # ~/.bashrc
  dirnote init fish | source     # ~/.config/fish/config.fish

The zsh hook uses add-zsh-hook chpwd, the bash hook wraps PROMPT_COMMAND
(keeping whatever is already there) and the fish hook watches $PWD.

  --inherit   pass --inherit to the hook: show a project's note when you cd
              into any directory inside it from outside
  --plain     pass --plain to the hook: no box, no colors
`

// noteJSON is one note in --json output.
type noteJSON struct {
	Dir       string    `json:"dir"`
	Source    string    `json:"source"` // "store" or "local"
	File      string    `json:"file"`
	Text      string    `json:"text"`
	Created   time.Time `json:"created,omitzero"`
	Updated   time.Time `json:"updated,omitzero"`
	Inherited bool      `json:"inherited"`
}

func (c *cli) show(args []string) error {
	flags := newFlags("show")
	dirFlag := flags.String("d", "", "")
	hook := flags.Bool("hook", false, "")
	plain := flags.Bool("plain", false, "")
	asJSON := flags.Bool("json", false, "")
	inherit := flags.Bool("inherit", envBool("DIRNOTE_INHERIT"), "")
	from := flags.String("from", "", "")
	if err := noArgs(flags, args); err != nil {
		return err
	}
	if *hook && *asJSON {
		return errUsage{"--hook and --json cannot be combined"}
	}
	opt := notes.Options{Inherit: *inherit}
	if *from != "" {
		if !*hook {
			return errUsage{"--from only applies with --hook"}
		}
		f, err := notes.Resolve(*from)
		if err != nil {
			return err
		}
		opt.From = f
	}

	if *hook {
		// Hook mode runs on every cd: stay quiet unless there is something
		// to show, keep stdout clean for command substitutions, and never
		// make the shell report a failure.
		_, entries, err := findNotes(*dirFlag, opt)
		if err != nil {
			fmt.Fprintf(c.stderr, "dirnote: %v\n", err)
		}
		if len(entries) > 0 {
			printSections(c.stderr, sections(entries), *plain)
		}
		return nil
	}

	dir, entries, err := findNotes(*dirFlag, opt)
	if err != nil && len(entries) == 0 {
		return err
	}
	if *asJSON {
		out := []noteJSON{}
		for _, e := range entries {
			out = append(out, noteJSON{e.Dir, string(e.Source), e.File, e.Text, e.Created, e.Updated, e.Inherited})
		}
		if err := writeJSON(c.stdout, out); err != nil {
			return err
		}
	} else if len(entries) > 0 {
		printSections(c.stdout, sections(entries), *plain)
	}
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return errNoNote{dir}
	}
	return nil
}

// findNotes returns the resolved directory named by dirFlag and the notes
// that apply to it.
func findNotes(dirFlag string, opt notes.Options) (string, []notes.Entry, error) {
	dir, err := targetDir(dirFlag, false)
	if err != nil {
		return "", nil, err
	}
	st, err := loadStore()
	if err != nil {
		return "", nil, err
	}
	entries, err := notes.Collect(st, dir, opt)
	return dir, entries, err
}

// sections labels each note with its directory, or with the .dirnote file
// it came from.
func sections(entries []notes.Entry) []render.Section {
	secs := make([]render.Section, len(entries))
	for i, e := range entries {
		label := e.Dir
		if e.Source == notes.SourceLocal {
			label = e.File
		}
		secs[i] = render.Section{Label: tildify(label), Text: e.Text}
	}
	return secs
}

func printSections(w io.Writer, secs []render.Section, plain bool) {
	if plain {
		fmt.Fprint(w, render.Plain(secs))
		return
	}
	fmt.Fprint(w, render.Box(secs, render.Options{Width: termWidth(w), Color: useColor(w)}))
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (c *cli) initShell(args []string) error {
	flags := newFlags("init")
	inherit := flags.Bool("inherit", false, "")
	plain := flags.Bool("plain", false, "")
	pos, err := parse(flags, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errUsage{"expected one shell: " + strings.Join(shell.Shells, ", ")}
	}
	var hookFlags []string
	if *inherit {
		hookFlags = append(hookFlags, "--inherit")
	}
	if *plain {
		hookFlags = append(hookFlags, "--plain")
	}
	script, err := shell.Script(pos[0], hookFlags...)
	if err != nil {
		return errUsage{err.Error()}
	}
	fmt.Fprint(c.stdout, script)
	return nil
}

// envBool reads a yes/no environment variable such as DIRNOTE_INHERIT=1.
func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
