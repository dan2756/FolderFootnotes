# dirnote

Leave a note on a directory; read it the next time you're there.

`dirnote` keeps short per-folder notes ("run `make dev` first, needs docker")
in one central file, keyed by the directory's real path.

## Install

```sh
go install github.com/dan2756/dirnote@latest
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
dirnote ls                                   # every note, with its directory
dirnote rm                                   # delete the note for this dir
dirnote prune -n                             # which notes point at deleted dirs?
dirnote prune                                # ...and drop them
```

`ls` marks directories that no longer exist with `(missing)`. `prune` only
drops a note when its directory is definitely gone; a permission error or an
unreachable mount counts as "still there".

`edit` runs `$VISUAL`, then `$EDITOR`, then `vi`, through `sh -c`, so values
like `code --wait` work. Saving an empty file deletes the note.

### Notes that ship with a repo

A directory can also carry its own note in a `.dirnote` file, so a project can
tell everyone who clones it how to get started. `--local` makes `add`, `set`,
`edit` and `rm` work on that file instead of the central store:

```sh
dirnote add --local "Node 20 only. Run make seed before the tests."
git add .dirnote
```

When both exist, `show` (and the hook) prints both, the central note first:

```
╭─ ~/src/api ──────────────────────────────────────╮
│ my staging creds are in 1password                │
├─ ~/src/api/.dirnote ─────────────────────────────┤
│ Node 20 only. Run make seed before the tests.    │
╰──────────────────────────────────────────────────╯
```

A `.dirnote` file is written by whoever wrote the repo, so dirnote treats it
as untrusted text: escape sequences and control characters are stripped before
anything is printed, only regular files are read (no FIFOs or devices), and
files over 64 KiB are refused.

### JSON

`show --json` and `ls --json` print arrays, for scripts and status lines.
`source` is `store` or `local` (a `.dirnote` file); `created` and `updated`
are only known for notes in the store.

```sh
$ dirnote show --json
[
  {
    "dir": "/home/dj/src/api",
    "source": "store",
    "file": "/home/dj/.local/share/dirnote/notes.json",
    "text": "run make dev first, needs docker",
    "created": "2026-06-03T19:55:12Z",
    "updated": "2026-06-03T19:55:12Z"
  }
]
$ dirnote ls --json | jq -r '.[] | select(.missing) | .dir'
/home/dj/src/old-prototype
```

### Exit codes

| code | meaning |
| ---- | ------- |
| 0 | success |
| 1 | `show` or `rm` found no note |
| 2 | bad usage, or an error such as an unreadable store |

`show --hook` always exits 0.

Notes are drawn in a box that wraps to the terminal width; list items keep a
hanging indent when they wrap. The directory label is dimmed on terminals that
support it. Use `--plain` for no box and no color, and set `NO_COLOR` to keep
the box but drop the escapes.


## Shell hook

To have the note pop up whenever you `cd` into a directory, load the hook for
your shell:

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
  An existing `PROMPT_COMMAND` is kept.
- **fish** uses a function with `--on-variable PWD`.

Each one calls `dirnote show --hook --from "$previous_dir"`, which prints
nothing when the directory has no note, writes notes to **stderr** (so
`$(cd dir && pwd)` style command substitutions are never polluted) and always
exits 0. The hook also runs once when the shell starts, so a new terminal
opened in a project shows its note. The zsh and fish hooks skip subshells and
command substitutions; the bash hook keeps `$?` intact for prompts that show
the last exit status.

Flags given to `init` are passed on to the hook:

```sh
eval "$(dirnote init zsh --inherit)"   # see below
eval "$(dirnote init bash --plain)"    # no box, no colors
```

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

A directory's own note is shown every time you `cd` into it.

## Configuration

| variable | effect |
| -------- | ------ |
| `DIRNOTE_FILE` | path of the note store (default `$XDG_DATA_HOME/dirnote/notes.json`) |
| `XDG_DATA_HOME` | base for the default store (default `~/.local/share`) |
| `DIRNOTE_INHERIT` | `1`/`true`/`yes`/`on` turns on `--inherit` for `show` |
| `NO_COLOR` | no ANSI colors |
| `COLUMNS` | width to wrap to when the output is not a terminal (default 80) |
| `VISUAL`, `EDITOR` | editor for `dirnote edit` |

The store is a plain JSON file keyed by absolute, symlink-resolved directory
paths, so `~/src/api` and a symlink pointing at it share one note. Every
write goes to a temporary file that is renamed over the old one, so a crash or
a second shell writing at the same time can never leave a half-written store.
