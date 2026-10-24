//go:build !unix

package notes

import "os"

func openLocal(path string) (*os.File, error) {
	return os.Open(path)
}

func isSymlinkOpen(error) bool { return false }
