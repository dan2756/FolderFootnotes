package notes

import (
	"io/fs"
	"os"
	"path/filepath"
)

// writeFileAtomic replaces path with data by writing a temporary file next
// to it and renaming it into place. An existing file keeps its permissions.
// A symlink at path is replaced, not followed; callers that want to write
// through one resolve it first. (.dirnote files deliberately do not: one in
// a cloned repository could point anywhere.)
func writeFileAtomic(path string, data []byte, perm fs.FileMode) (err error) {
	if fi, err := os.Lstat(path); err == nil && fi.Mode().IsRegular() {
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

// linkTarget follows path through any symlinks, including a dangling last
// one, and returns the file they end at.
func linkTarget(path string) string {
	for range 40 { // the usual limit on symlink hops
		fi, err := os.Lstat(path)
		if err != nil || fi.Mode()&fs.ModeSymlink == 0 {
			return path
		}
		target, err := os.Readlink(path)
		if err != nil {
			return path
		}
		if !filepath.IsAbs(target) {
			// The target starts from the directory the link really is
			// in. That differs from the lexical parent when path runs
			// through a symlinked directory and the target uses "..".
			target = filepath.Join(resolveAbs(filepath.Dir(path)), target)
		}
		path = target
	}
	return path
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
