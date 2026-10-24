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
// is no such file, it is empty, or it is not a regular file (a directory, a
// FIFO, a device). A symlink is an error: the hook runs inside cloned
// repositories, and the link could point at any file the user can read.
func ReadLocal(dir string) (text string, ok bool, err error) {
	path := LocalPath(dir)
	fi, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
			return "", false, nil
		}
		return "", false, err
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		return "", false, fmt.Errorf("%s is a symlink, not reading it", path)
	}
	if !fi.Mode().IsRegular() {
		return "", false, nil
	}
	// O_NOFOLLOW|O_NONBLOCK: the file can be replaced between the lstat and
	// the open, and a FIFO must not block the shell hook.
	f, err := openLocal(path)
	if err != nil {
		if isSymlinkOpen(err) {
			return "", false, fmt.Errorf("%s is a symlink, not reading it", path)
		}
		return "", false, err
	}
	defer f.Close()
	fi, err = f.Stat()
	if err != nil {
		return "", false, err
	}
	if !fi.Mode().IsRegular() {
		return "", false, nil
	}
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
