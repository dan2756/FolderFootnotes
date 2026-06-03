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
# run make dev first, needs docker
# staging creds are in 1password

dirnote set -d ~/src/web "pnpm, not npm"     # replace the note for another dir
dirnote ls                                   # every note, with its directory
dirnote rm                                   # delete the note for this dir
```

Notes live in `$XDG_DATA_HOME/dirnote/notes.json`
(`~/.local/share/dirnote/notes.json` by default). Set `DIRNOTE_FILE` to use
another file.
