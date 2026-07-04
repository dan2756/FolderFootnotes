package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dan2756/dirnote/internal/notes"
	"github.com/dan2756/dirnote/internal/render"
	"github.com/dan2756/dirnote/internal/shell"
)

const showHelp = `Usage: dirnote show [-d DIR] [--hook] [--plain] [--json]

Print the note for the current directory in a box sized to the terminal.
Exits 1 if there is none.

  -d DIR    act on DIR instead of the current directory
  --hook    mode for shell hooks: print to stderr, say nothing and exit 0
            when there is no note
  --plain   no box and no colors, just the directory and the note
  --json    print a JSON array of notes (empty when there is none)

Colors are left out when the output is not a terminal or NO_COLOR is set.
`

const initHelp = `Usage: dirnote init zsh|bash|fish

Print a hook that runs 'dirnote show --hook' whenever the working directory
changes. Load it from your shell's startup file:

  eval "$(dirnote init zsh)"     # ~/.zshrc
  eval "$(dirnote init bash)"    # ~/.bashrc
  dirnote init fish | source     # ~/.config/fish/config.fish

The zsh hook uses add-zsh-hook chpwd, the bash hook wraps PROMPT_COMMAND
(keeping whatever is already there) and the fish hook watches $PWD.
`

// noteJSON is one note in --json output.
type noteJSON struct {
	Dir     string    `json:"dir"`
	Source  string    `json:"source"`
	Text    string    `json:"text"`
	Created time.Time `json:"created,omitzero"`
	Updated time.Time `json:"updated,omitzero"`
}

func (c *cli) show(args []string) error {
	flags := newFlags("show")
	dirFlag := flags.String("d", "", "")
	hook := flags.Bool("hook", false, "")
	plain := flags.Bool("plain", false, "")
	asJSON := flags.Bool("json", false, "")
	if err := noArgs(flags, args); err != nil {
		return err
	}
	if *hook && *asJSON {
		return errUsage{"--hook and --json cannot be combined"}
	}

	if *hook {
		// Hook mode runs on every cd: stay quiet unless there is something
		// to show, keep stdout clean for command substitutions, and never
		// make the shell report a failure.
		dir, n, err := findNote(*dirFlag)
		if err != nil {
			fmt.Fprintf(c.stderr, "dirnote: %v\n", err)
		} else if n != nil {
			printSections(c.stderr, []render.Section{{Label: tildify(dir), Text: n.Text}}, *plain)
		}
		return nil
	}

	dir, n, err := findNote(*dirFlag)
	if err != nil {
		return err
	}
	if *asJSON {
		out := []noteJSON{}
		if n != nil {
			out = append(out, noteJSON{Dir: dir, Source: "store", Text: n.Text, Created: n.Created, Updated: n.Updated})
		}
		if err := writeJSON(c.stdout, out); err != nil {
			return err
		}
	} else if n != nil {
		printSections(c.stdout, []render.Section{{Label: tildify(dir), Text: n.Text}}, *plain)
	}
	if n == nil {
		return errNoNote{dir}
	}
	return nil
}

// findNote returns the resolved directory named by dirFlag and its note,
// which is nil if there is none.
func findNote(dirFlag string) (string, *notes.Note, error) {
	dir, err := targetDir(dirFlag, false)
	if err != nil {
		return "", nil, err
	}
	st, err := loadStore()
	if err != nil {
		return "", nil, err
	}
	n, _ := st.Get(dir)
	return dir, n, nil
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
	pos, err := parse(flags, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errUsage{"expected one shell: " + strings.Join(shell.Shells, ", ")}
	}
	script, err := shell.Script(pos[0])
	if err != nil {
		return errUsage{err.Error()}
	}
	fmt.Fprint(c.stdout, script)
	return nil
}
