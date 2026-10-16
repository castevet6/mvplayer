package browser

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestListAndFilter(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"zeta.MP3", "Alpha.mkv", "notes.txt", ".hidden.mp4"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "Folder"), 0o700); err != nil {
		t.Fatal(err)
	}
	entries, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	if want := []string{"Alpha.mkv", "Folder", "zeta.MP3"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %#v, want %#v", names, want)
	}
	if !entries[1].Dir || !entries[0].Media || !entries[2].Media {
		t.Fatalf("entry types not identified: %#v", entries)
	}
	filtered := Filter(entries, "ZETA")
	if len(filtered) != 1 || filtered[0].Name != "zeta.MP3" {
		t.Fatalf("filter result = %#v", filtered)
	}
}

func TestListMissingDirectory(t *testing.T) {
	if _, err := List(filepath.Join(t.TempDir(), "gone")); err == nil {
		t.Fatal("List() expected missing-directory error")
	}
}
