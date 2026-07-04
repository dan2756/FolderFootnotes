package main

import (
	"fmt"
	"strings"
	"time"
)

const lsHelp = `Usage: dirnote ls [--json]

List every note in the store with its directory. Directories that no longer
exist are flagged as missing.

  --json   print a JSON array instead
`

const pruneHelp = `Usage: dirnote prune [-n]

Delete notes whose directories no longer exist.

  -n, --dry-run   only list what would be deleted
`

type lsJSON struct {
	Dir     string    `json:"dir"`
	Text    string    `json:"text"`
	Created time.Time `json:"created,omitzero"`
	Updated time.Time `json:"updated,omitzero"`
	Missing bool      `json:"missing"`
}

func (c *cli) ls(args []string) error {
	flags := newFlags("ls")
	asJSON := flags.Bool("json", false, "")
	if err := noArgs(flags, args); err != nil {
		return err
	}
	st, err := loadStore()
	if err != nil {
		return err
	}

	if *asJSON {
		out := []lsJSON{}
		for _, dir := range st.Dirs() {
			n := st.Notes[dir]
			out = append(out, lsJSON{dir, n.Text, n.Created, n.Updated, !dirExists(dir)})
		}
		return writeJSON(c.stdout, out)
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

func (c *cli) prune(args []string) error {
	flags := newFlags("prune")
	dryRun := flags.Bool("n", false, "")
	flags.BoolVar(dryRun, "dry-run", false, "")
	if err := noArgs(flags, args); err != nil {
		return err
	}
	st, err := loadStore()
	if err != nil {
		return err
	}
	gone := st.Prune(dirExists)
	if len(gone) == 0 {
		fmt.Fprintln(c.stderr, "nothing to prune")
		return nil
	}
	verb := "deleted"
	if *dryRun {
		verb = "would delete"
	} else if err := st.Save(); err != nil {
		return err
	}
	for _, dir := range gone {
		fmt.Fprintf(c.stdout, "%s note for %s\n", verb, tildify(dir))
	}
	return nil
}
