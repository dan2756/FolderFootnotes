//go:build unix

package notes

import (
	"errors"
	"os"
	"syscall"
)

// openLocal opens path for reading without following a symlink and without
// blocking if it is a FIFO.
func openLocal(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}

func isSymlinkOpen(err error) bool {
	return errors.Is(err, syscall.ELOOP)
}
