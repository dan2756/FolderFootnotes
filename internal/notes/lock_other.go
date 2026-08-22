//go:build !unix

package notes

import "os"

// flock does nothing where flock(2) is missing. dirnote supports Linux and
// macOS; elsewhere concurrent writers can still overwrite each other.
func flock(*os.File) error { return nil }
