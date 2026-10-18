package browser

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	target := filepath.Join(t.TempDir(), "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "LinkedFolder")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	entries, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	if want := []string{"Alpha.mkv", "Folder", "LinkedFolder", "zeta.MP3"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %#v, want %#v", names, want)
	}
	if !entries[1].Dir || !entries[2].Dir || !entries[0].Media || !entries[3].Media {
		t.Fatalf("entry types not identified: %#v", entries)
	}
	filtered := Filter(entries, "ZETA")
	if len(filtered) != 1 || filtered[0].Name != "zeta.MP3" {
		t.Fatalf("filter result = %#v", filtered)
	}
}

func TestListRecursivePathsOrderAndFiltering(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"album/live", "other", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"zeta.mp3", "song.flac", "album/song.flac", "album/live/song.flac", "other/Song.flac", ".hidden/secret.mp4", "album/no.txt"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(root, "album"), filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(root, filepath.Join(root, "album/live/root-link")); err != nil {
		t.Skipf("symlink cycle unavailable: %v", err)
	}
	entries, warning, err := ListRecursive(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if warning != "" {
		t.Fatalf("unexpected warning: %q", warning)
	}
	want := []string{"album/", "linked/", "other/", "album/live/song.flac", "album/song.flac", "other/Song.flac", "song.flac", "zeta.mp3"}
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.RelativePath
		if entry.Dir {
			name += "/"
		}
		got = append(got, name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recursive entries = %#v, want %#v", got, want)
	}
	filtered := Filter(entries, "LIVE/SONG")
	if len(filtered) != 1 || filtered[0].Path != filepath.Join(root, "album/live/song.flac") {
		t.Fatalf("relative path filter = %#v", filtered)
	}
	linkedEntries, warning, err := ListRecursive(context.Background(), filepath.Join(root, "linked"))
	if err != nil || warning != "" {
		t.Fatalf("listing explicitly selected symlink root: warning=%q err=%v", warning, err)
	}
	if len(linkedEntries) != 3 || !linkedEntries[0].Dir || linkedEntries[2].Path != filepath.Join(root, "linked/song.flac") {
		t.Fatalf("symlink-root entries = %#v", linkedEntries)
	}
}

func TestListRecursiveCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := ListRecursive(ctx, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatalf("ListRecursive error = %v, want context.Canceled", err)
	}
}

func TestListRecursiveRetainsResultsOnDescendantFailure(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"broken", "readable"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	media := filepath.Join(root, "readable", "keep.mp4")
	if err := os.WriteFile(media, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	readDir := func(path string) ([]os.DirEntry, error) {
		if path == filepath.Join(root, "broken") {
			return nil, os.ErrPermission
		}
		return os.ReadDir(path)
	}
	entries, warning, err := listRecursiveWithReadDir(context.Background(), root, readDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 || entries[2].Path != media {
		t.Fatalf("entries after partial failure = %#v", entries)
	}
	if !strings.Contains(warning, filepath.Join(root, "broken")) || !strings.Contains(warning, "skipped 1 subtree") {
		t.Fatalf("partial warning = %q", warning)
	}
}

func TestListMissingDirectory(t *testing.T) {
	if _, err := List(filepath.Join(t.TempDir(), "gone")); err == nil {
		t.Fatal("List() expected missing-directory error")
	}
}
