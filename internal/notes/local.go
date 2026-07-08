package notes

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// LocalName is the file a directory can carry its own note in, so that a
// repository can ship one.
const LocalName = ".dirnote"

// maxLocalSize caps how much of a .dirnote file is read. The hook runs on
// every cd, including into freshly cloned repositories, so a huge file must
// not stall the shell.
const maxLocalSize = 64 << 10

// LocalPath returns the path of dir's .dirnote file.
func LocalPath(dir string) string { return filepath.Join(dir, LocalName) }

// ReadLocal returns the note in dir's .dirnote file. ok is false when there
// is no such file or it is empty. Anything that is not a regular file (a
// directory, a FIFO, a symlink to a device) is ignored.
func ReadLocal(dir string) (text string, ok bool, err error) {
	path := LocalPath(dir)
	fi, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
			return "", false, nil
		}
		return "", false, err
	}
	if !fi.Mode().IsRegular() {
		return "", false, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxLocalSize+1))
	if err != nil {
		return "", false, err
	}
	if len(b) > maxLocalSize {
		return "", false, fmt.Errorf("%s is over %d KiB, not reading it", path, maxLocalSize>>10)
	}
	text = strings.TrimSpace(string(b))
	return text, text != "", nil
}

// WriteLocal replaces dir's .dirnote file with text.
func WriteLocal(dir, text string) error {
	return writeFileAtomic(LocalPath(dir), []byte(text+"\n"), 0o644)
}

// RemoveLocal deletes dir's .dirnote file and reports whether there was one.
func RemoveLocal(dir string) (bool, error) {
	err := os.Remove(LocalPath(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}
