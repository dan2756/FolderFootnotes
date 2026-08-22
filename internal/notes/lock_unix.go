//go:build unix

package notes

import (
	"os"
	"syscall"
)

// flock waits for an exclusive advisory lock on f.
func flock(f *os.File) error {
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
		if err != syscall.EINTR {
			return err
		}
	}
}
