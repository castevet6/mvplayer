package tui

import (
	"errors"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"mvplayer/internal/browser"
)

func TestDirectoryResultsIgnoreStaleRequests(t *testing.T) {
	dir := t.TempDir()
	m := NewWithLister(dir, func(string) ([]browser.Entry, error) { return nil, nil })
	stale := directoryResult{request: m.request - 1, path: dir, entries: []browser.Entry{{Name: "old.mp4"}}}
	updated, _ := m.Update(stale)
	if got := updated.(Model).entries; len(got) != 0 {
		t.Fatalf("stale response replaced entries: %#v", got)
	}
	current := directoryResult{request: m.request, path: dir, entries: []browser.Entry{{Name: "new.mp4", Path: filepath.Join(dir, "new.mp4"), Media: true}}}
	updated, _ = m.Update(current)
	if got := updated.(Model).visible; len(got) != 1 || got[0].Name != "new.mp4" {
		t.Fatalf("current response not applied: %#v", got)
	}
}

func TestNavigationFilteringAndFileSelection(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	media := filepath.Join(root, "movie.mp4")
	m := NewWithLister(root, func(path string) ([]browser.Entry, error) {
		if path == root {
			return []browser.Entry{{Name: "sub", Path: sub, Dir: true}, {Name: "movie.mp4", Path: media, Media: true}}, nil
		}
		return nil, errors.New("unreadable")
	})
	cmd := m.Init()
	updated, _ := m.Update(cmd())
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	m = updated.(Model)
	updated, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("Enter did not emit file selection")
	}
	if selected, ok := cmd().(FileSelectedMsg); !ok || selected.Path != media {
		t.Fatalf("selection = %#v", cmd())
	}

	// Open the directory and verify that a listing failure is shown while the
	// model remains usable.
	m.cursor = 0
	updated, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.err == nil || m.browseDir != sub {
		t.Fatalf("directory failure state = dir %q, error %v", m.browseDir, m.err)
	}

	// Filter typing is case insensitive and can be cleared.
	m.filtering = true
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	m = updated.(Model)
	if m.query != "x" {
		t.Fatalf("query = %q", m.query)
	}
	if len(m.visible) != 0 {
		t.Fatalf("nonmatching filter returned entries: %#v", m.visible)
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	m = updated.(Model)
	updated, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	m = updated.(Model)
	if m.browseDir != root || cmd == nil {
		t.Fatalf("parent navigation = dir %q, command %v", m.browseDir, cmd != nil)
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.err != nil || len(m.visible) != 2 {
		t.Fatalf("parent listing did not recover: error %v, entries %#v", m.err, m.visible)
	}
}
