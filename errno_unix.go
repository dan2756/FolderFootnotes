//go:build unix

package main

import (
	"errors"
	"syscall"
)

// isNotADirectory reports whether err is ENOTDIR: a parent of the path is
// a file, so the path cannot be a directory.
func isNotADirectory(err error) bool {
	return errors.Is(err, syscall.ENOTDIR)
}
