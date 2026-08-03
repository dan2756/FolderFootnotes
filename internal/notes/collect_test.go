package notes

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// tree builds
//
//	root/proj          store note "proj"
//	root/proj/src      .dirnote "src"
//	root/proj/src/pkg  no note
//	root/other         no note
func tree(t *testing.T) (*Store, string) {
	t.Helper()
	root := tempDir(t)
	for _, d := range []string{"proj/src/pkg", "other"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	st := &Store{Path: filepath.Join(root, "notes.json"), Notes: map[string]*Note{}}
	st.Set(filepath.Join(root, "proj"), "proj", time.Now())
	if err := WriteLocal(filepath.Join(root, "proj", "src"), "src"); err != nil {
		t.Fatal(err)
	}
	return st, root
}

func texts(es []Entry) []string {
	out := []string{}
	for _, e := range es {
		out = append(out, e.Text)
	}
	return out
}

func TestCollectInherit(t *testing.T) {
	st, root := tree(t)
	p := func(rel string) string {
		if rel == "" {
			return ""
		}
		return filepath.Join(root, rel)
	}

	tests := []struct {
		name    string
		dir     string
		inherit bool
		from    string
		want    []string
	}{
		{"own note only", "proj", false, "", []string{"proj"}},
		{"no inherit, nothing here", "proj/src/pkg", false, "", []string{}},
		{"inherit, closest first", "proj/src/pkg", true, "", []string{"src", "proj"}},
		{"entered from outside", "proj/src/pkg", true, "other", []string{"src", "proj"}},
		{"entered src from proj", "proj/src", true, "proj", []string{"src"}},
		{"moved within src", "proj/src/pkg", true, "proj/src", []string{}},
		{"came up from a child", "proj/src", true, "proj/src/pkg", []string{"src"}},
		{"came up into proj", "proj", true, "proj/src", []string{"proj"}},
		{"same dir again", "proj", true, "proj", []string{}},
		{"same dir, no inherit", "proj", false, "proj", []string{}},
		{"jump between subtrees", "proj/src/pkg", true, "proj/other-sibling", []string{"src"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Collect(st, p(tt.dir), Options{Inherit: tt.inherit, From: p(tt.from)})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(texts(got), tt.want) {
				t.Errorf("Collect(%s, from %q) = %q, want %q", tt.dir, tt.from, texts(got), tt.want)
			}
		})
	}
}

func TestCollectMarksInherited(t *testing.T) {
	st, root := tree(t)
	got, err := Collect(st, filepath.Join(root, "proj", "src"), Options{Inherit: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	if got[0].Inherited || got[0].Source != SourceLocal || got[0].Dir != filepath.Join(root, "proj", "src") {
		t.Errorf("own note: %+v", got[0])
	}
	if !got[1].Inherited || got[1].Source != SourceStore || got[1].Dir != filepath.Join(root, "proj") {
		t.Errorf("parent note: %+v", got[1])
	}
}

func TestWithin(t *testing.T) {
	tests := []struct {
		path, dir string
		want      bool
	}{
		{"/a/b", "/a", true},
		{"/a", "/a", true},
		{"/ab", "/a", false},
		{"/a", "/a/b", false},
		{"/anything", "/", true},
		{"/", "/", true},
	}
	for _, tt := range tests {
		if got := Within(tt.path, tt.dir); got != tt.want {
			t.Errorf("Within(%q, %q) = %v, want %v", tt.path, tt.dir, got, tt.want)
		}
	}
}
