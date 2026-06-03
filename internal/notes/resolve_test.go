package notes

import (
	"os"
	"path/filepath"
	"testing"
)

// tempDir returns a resolved temp dir; on macOS t.TempDir lives under
// /var, which is a symlink to /private/var.
func tempDir(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestResolve(t *testing.T) {
	root := tempDir(t)
	proj := filepath.Join(root, "real", "project")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(filepath.Join(root, "real"), link); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name, in, want string
	}{
		{"plain", proj, proj},
		{"unclean", proj + "/../project/./", proj},
		{"symlinked parent", filepath.Join(link, "project"), proj},
		{"missing leaf", filepath.Join(link, "project", "gone"), filepath.Join(proj, "gone")},
		{"missing subtree", filepath.Join(link, "a", "b"), filepath.Join(root, "real", "a", "b")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("Resolve(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestResolveRelative(t *testing.T) {
	root := tempDir(t)
	os.Mkdir(filepath.Join(root, "sub"), 0o755)
	t.Chdir(root)
	got, err := Resolve("sub")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "sub"); got != want {
		t.Errorf("Resolve(sub) = %q, want %q", got, want)
	}
}
