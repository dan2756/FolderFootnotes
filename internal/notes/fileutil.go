package notes

import (
	"io/fs"
	"os"
	"path/filepath"
)

// writeFileAtomic replaces path with data by writing a temporary file next
// to it and renaming it into place. An existing file keeps its permissions.
func writeFileAtomic(path string, data []byte, perm fs.FileMode) (err error) {
	if fi, err := os.Stat(path); err == nil {
		perm = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Chmod(perm); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Resolve returns the absolute, symlink-free form of path. When the path
// does not exist (or no longer does), the deepest existing ancestor is
// resolved and the rest is kept as written, so notes for deleted
// directories can still be looked up.
func Resolve(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return resolveAbs(abs), nil
}

func resolveAbs(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	parent := filepath.Dir(p)
	if parent == p {
		return p
	}
	return filepath.Join(resolveAbs(parent), filepath.Base(p))
}
