# dirnote

Leave a note on a directory; see it the next time you `cd` in.

You come back to a project after three weeks and can't remember that it needs
`make dev` before anything works, or that the tests want docker running.
`dirnote` lets you pin that kind of reminder to the directory itself, and a
small shell hook prints it in a box whenever you arrive there:

```
~ $ cd ~/src/api
╭─ ~/src/api ──────────────────────╮
│ run make dev first, needs docker │
╰──────────────────────────────────╯
api $
```

Notes live in one central file keyed by the directory's real path, or, for
notes a repository should ship to everyone, in a `.dirnote` file inside it.

## Install

```sh
go install github.com/dan2756/dirnote@latest
```

Then add the hook to your shell (see [Shell hook](#shell-hook)):

```sh
echo 'eval "$(dirnote init zsh)"' >> ~/.zshrc
```

## Usage

```sh
cd ~/src/api
dirnote add "run make dev first, needs docker"
dirnote add staging creds are in 1password   # appends a line; quotes optional
dirnote show
# ╭─ ~/src/api ──────────────────────╮
# │ run make dev first, needs docker │
# │ staging creds are in 1password   │
# ╰──────────────────────────────────╯

dirnote set -d ~/src/web "pnpm, not npm"     # replace the note for another dir
dirnote edit                                 # open it in $VISUAL / $EDITOR
echo "ask Sam about the S3 bucket" | dirnote add   # text from stdin
dirnote ls                                   # every note, with its directory
dirnote rm                                   # delete the note for this dir
dirnote prune -n                             # which notes point at deleted dirs?
dirnote prune                                # ...and drop them
```

| command | what it does |
| ------- | ------------ |
| `add [-d DIR] [--local] TEXT...` | append a line to the note (creating it); reads stdin if there is no TEXT |
| `set [-d DIR] [--local] TEXT...` | replace the note |
| `edit [-d DIR] [--local]` | open the note in `$VISUAL`, `$EDITOR` or `vi`; saving it empty deletes it |
| `show [-d DIR] [--inherit] [--plain \| --json]` | print the notes for the directory (plain `dirnote` does this too) |
| `rm [-d DIR] [--local]` | delete the note; DIR may already be gone |
| `ls [--json]` | list every note in the store, flagging directories that no longer exist |
| `prune [-n \| --dry-run]` | delete notes whose directories are gone |
| `init zsh\|bash\|fish [--inherit] [--plain]` | print the shell hook |

Every command works on the current directory unless `-d DIR` is given. Flags
can go before or after the text. Text that starts with a dash and a space,
like `- seed the db first`, is taken as text; for anything else that looks
like a flag, put `--` in front (`dirnote add -- -v makes it verbose`).

`prune` only drops a note when its directory is definitely gone; a permission
error or an unreachable mount counts as "still there". `edit` runs the editor
through `sh -c`, so values like `EDITOR="code --wait"` work. If saving that
edit fails, the edited text is left in a temporary file and the path is
printed.

### Output

Notes are drawn in a rounded box that wraps to the terminal width. List items
(`- `, `* `, `1. `) keep a hanging indent when they wrap, and long paths in
the header are shortened from the left. On a terminal the directory label is
dimmed.

- `--plain` drops the box and colors: the directory, then the note indented.
- `NO_COLOR` (any value) keeps the box but drops the escapes. Output that is
  not a terminal never gets colors.
- `show` writes to **stdout**; `show --hook` writes to **stderr**.

## Shell hook

```sh
# ~/.zshrc
eval "$(dirnote init zsh)"

# ~/.bashrc
eval "$(dirnote init bash)"

# ~/.config/fish/config.fish
dirnote init fish | source
```

- **zsh** registers a `chpwd` hook with `add-zsh-hook`.
- **bash** has no cd hook, so a function added to the front of
  `PROMPT_COMMAND` runs `dirnote` when `$PWD` changed since the last prompt.
  An existing `PROMPT_COMMAND` (string or array) is kept, and `$?` is passed
  through untouched for prompts that show the last exit status.
- **fish** uses a function with `--on-variable PWD`.

Each one calls `dirnote show --hook --from "$previous_dir"`. In hook mode
dirnote:

- prints nothing at all when there is no note, and takes a few milliseconds
  either way;
- writes to **stderr**, so `$(cd dir && pwd)` style command substitutions are
  never polluted (the zsh and fish hooks skip subshells and command
  substitutions altogether);
- always exits 0; a broken store is reported in one line and the shell
  carries on.

The hook also runs once when the shell starts, so a new terminal opened in a
project shows its note. Flags given to `init` are passed on to the hook:

```sh
eval "$(dirnote init zsh --inherit)"   # see below
eval "$(dirnote init bash --plain)"    # no box, no colors
```

The hook calls `dirnote` through `$PATH`, so make sure the install location
(usually `~/go/bin`) is on it.

## Notes from parent directories

With `--inherit` (or `DIRNOTE_INHERIT=1` in the environment), `show` also
prints the notes of every parent directory, closest first. So a note on
`~/src/api` is still shown when you open a terminal in `~/src/api/internal/db`.

In hook mode that would get noisy, so a parent's note is only shown when you
*enter its subtree from outside*:

```
~ $ cd ~/src/api/internal/db      # shows the ~/src/api note: you just arrived
db $ cd ../http                   # nothing: still inside ~/src/api
http $ cd ~                       # nothing
~ $ cd ~/src/api                  # its own note, as always
```

A directory's own note is shown every time you `cd` into it; `cd .` shows
nothing.

## Notes that ship with a repo

A directory can carry its own note in a `.dirnote` file, so a project can tell
everyone who clones it how to get started. `--local` makes `add`, `set`,
`edit` and `rm` work on that file instead of the central store:

```sh
dirnote add --local "Node 20 only. Run make seed before the tests."
git add .dirnote
```

When both exist, `show` (and the hook) prints both, the central note first:

```
╭─ ~/src/api ───────────────────────────────────╮
│ my staging creds are in 1password             │
├─ ~/src/api/.dirnote ──────────────────────────┤
│ Node 20 only. Run make seed before the tests. │
╰───────────────────────────────────────────────╯
```

A `.dirnote` file is written by whoever wrote the repo, so dirnote treats it
as untrusted:

- escape sequences, control characters and bidi overrides are stripped
  before anything is printed, from the note and from directory names in
  messages alike;
- only regular files are read. Symlinks are refused (a `.dirnote` link in a
  cloned repository could point at any file); directories, FIFOs and devices
  are ignored. Files over 64 KiB are refused;
- writing with `--local` replaces a symlinked `.dirnote` rather than writing
  through it, and `add` and `edit` then start from an empty note instead of
  copying in the file the link points to.

`ls` and `prune` only cover the central store.

## JSON

`show --json` and `ls --json` print arrays, for scripts and status lines.
`source` is `store` or `local` (a `.dirnote` file), `created` and `updated`
are only known for notes in the store, and `inherited` marks notes from
parent directories. The text of a `.dirnote` is stripped of escape
sequences, control characters and bidi overrides here too; paths are given
exactly.

```sh
$ dirnote show --json
[
  {
    "dir": "/home/dj/src/api",
    "source": "store",
    "file": "/home/dj/.local/share/dirnote/notes.json",
    "text": "run make dev first, needs docker",
    "created": "2026-06-03T19:55:12Z",
    "updated": "2026-06-03T19:55:12Z",
    "inherited": false
  }
]
$ dirnote ls --json | jq -r '.[] | select(.missing) | .dir'
/home/dj/src/old-prototype
```

`show --json` prints `[]` (and exits 1) when there is no note.

## Exit codes

| code | meaning |
| ---- | ------- |
| 0 | success |
| 1 | `show` or `rm` found no note |
| 2 | bad usage, or an error such as an unreadable store or a failed editor |

`show --hook` always exits 0.

## Configuration

| variable | effect |
| -------- | ------ |
| `DIRNOTE_FILE` | path of the note store (default `$XDG_DATA_HOME/dirnote/notes.json`) |
| `XDG_DATA_HOME` | base for the default store (default `~/.local/share`) |
| `DIRNOTE_INHERIT` | `1`/`true`/`yes`/`on` turns on `--inherit` for `show` |
| `NO_COLOR` | no ANSI colors |
| `COLUMNS` | width to wrap to when the output is not a terminal (default 80) |
| `VISUAL`, `EDITOR` | editor for `dirnote edit` |

## Storage

The store is a plain, pretty-printed JSON file:

```json
{
  "version": 1,
  "notes": {
    "/home/dj/src/api": {
      "text": "run make dev first, needs docker",
      "created": "2026-06-03T19:55:12Z",
      "updated": "2026-06-03T19:55:12Z"
    }
  }
}
```

Keys are absolute, symlink-resolved paths, so `~/src/api` and a symlink
pointing at it share one note. Every write goes to a temporary file in the
same directory that is then renamed over the old one, so a crash never
leaves a half-written store. Commands that change the store hold a lock on
`notes.json.lock` next to it while they read, change and write it, so notes
saved from several shells at once are all kept; `edit` reads the store again
after the editor closes and changes only the note it edited. If
`notes.json` is itself a symlink (into a dotfiles repo, say), the file it
points to is replaced and the link is left alone. A new store is created
with mode 0600.

## Platform notes

dirnote runs on Linux and macOS.

- The default store is `~/.local/share/dirnote/notes.json` on macOS too, not
  `~/Library`; set `DIRNOTE_FILE` to keep it elsewhere.
- On macOS `/tmp` and `/var` are symlinks into `/private`. Because paths are
  resolved, a note added from `/tmp/x` is stored under `/private/tmp/x` and
  found from either spelling.
- Paths are compared exactly. On a case-insensitive volume (the macOS
  default) `cd ~/Src/API` reaches the same directory as `cd ~/src/api` under
  a different name, so stick to one spelling.
- The store is JSON, which cannot hold bytes that are not valid UTF-8.
  `add` and `set` refuse a directory whose name is not UTF-8 (a Latin-1
  `café`, say) and a note whose text is not UTF-8. A `.dirnote` file
  (`--local`) keeps those bytes as they are.
