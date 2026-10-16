package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"mvplayer/internal/browser"
	"mvplayer/internal/playback"
)

type fakePlayer struct {
	paths              []string
	err                error
	events             chan playback.State
	paused             []bool
	seeks              []float64
	volumes            []float64
	stops, fullscreens int
}

func (p *fakePlayer) Load(_ context.Context, path string) error {
	p.paths = append(p.paths, path)
	return p.err
}
func (p *fakePlayer) Close() error { return nil }
func (p *fakePlayer) Events() <-chan playback.State {
	if p.events == nil {
		p.events = make(chan playback.State, 8)
	}
	return p.events
}
func (p *fakePlayer) Pause(_ context.Context, v bool) error {
	p.paused = append(p.paused, v)
	return nil
}
func (p *fakePlayer) Seek(_ context.Context, v float64) error {
	p.seeks = append(p.seeks, v)
	return nil
}
func (p *fakePlayer) SetVolume(_ context.Context, v float64) error {
	p.volumes = append(p.volumes, v)
	return nil
}
func (p *fakePlayer) Stop(context.Context) error                 { p.stops++; return nil }
func (p *fakePlayer) Fullscreen(_ context.Context, v bool) error { p.fullscreens++; return nil }

func TestSelectedFileLoadsThroughPlayer(t *testing.T) {
	player := &fakePlayer{}
	m := New(t.TempDir(), "", player)
	m.selected = "/tmp/clip.mp4"
	m.status = playback.State{Ended: true, EndReason: "error", HasTime: true, HasDuration: true}
	updated, cmd := m.Update(FileSelectedMsg{Path: "/tmp/clip.mp4"})
	if cmd == nil {
		t.Fatal("selection did not schedule playback")
	}
	selected := updated.(Model)
	if selected.status.Ended || selected.status.HasTime || selected.status.HasDuration {
		t.Fatalf("new selection retained stale playback status: %#v", selected.status)
	}
	result := cmd()
	updated, _ = selected.Update(result)
	m = updated.(Model)
	if len(player.paths) != 1 || player.paths[0] != "/tmp/clip.mp4" || m.playing != "/tmp/clip.mp4" || m.selected != "" {
		t.Fatalf("load state: paths=%v playing=%q selected=%q", player.paths, m.playing, m.selected)
	}
}

func TestStaleEOFStateDoesNotClearReload(t *testing.T) {
	player := &fakePlayer{}
	path := "/tmp/repeat.mp4"
	m := New(t.TempDir(), "", player)
	m.pending, m.playing = path, path
	updated, cmd := m.Update(FileSelectedMsg{Path: path})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("reload did not schedule player load")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	updated, _ = m.Update(playerStateMsg(playback.State{})) // delayed EOF snapshot
	m = updated.(Model)
	if m.playing != path || m.pending != path {
		t.Fatalf("delayed EOF state cleared a fresh reload: playing=%q pending=%q", m.playing, m.pending)
	}
	updated, _ = m.Update(playerStateMsg(playback.State{Path: path, HasTime: true, Time: 0.1}))
	m = updated.(Model)
	updated, _ = m.Update(playerStateMsg(playback.State{})) // EOF of the new playback
	m = updated.(Model)
	if m.playing != "" || m.pending != "" {
		t.Fatalf("current EOF state was not applied: playing=%q pending=%q", m.playing, m.pending)
	}
}

func TestEnterReloadsAfterEOFIntegration(t *testing.T) {
	if os.Getenv("MVPLAYER_MPV_INTEGRATION") != "1" {
		t.Skip("set MVPLAYER_MPV_INTEGRATION=1 to run the real TUI/mpv check")
	}
	path := os.Getenv("MVPLAYER_MPV_FIXTURE")
	if path == "" {
		t.Skip("set MVPLAYER_MPV_FIXTURE to a three-second MP4")
	}
	args := []string{"--vo=null", "--ao=null"}
	if os.Getenv("MVPLAYER_MPV_VIDEO_OUTPUT") == "1" {
		args = nil
	}
	p := playback.NewMPV(args...)
	defer p.Close()
	m := New(t.TempDir(), "", p)
	m.visible = []browser.Entry{{Name: filepath.Base(path), Path: path, Media: true}}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			sawEnded := false
			deadline := time.NewTimer(8 * time.Second)
			for !sawEnded {
				select {
				case state := <-p.Events():
					updated, _ := m.Update(playerStateMsg(state))
					m = updated.(Model)
					if state.Ended && state.EndReason == "eof" {
						sawEnded = true
					}
				case <-deadline.C:
					t.Fatal("timed out waiting for natural EOF")
				case <-ctx.Done():
					deadline.Stop()
					t.Fatal(ctx.Err())
				}
			}
			deadline.Stop()
			if !sawEnded || !strings.Contains(m.View().Content, "Ended") {
				t.Fatal("natural EOF did not update the TUI to Ended")
			}
		}
		updated, selectCmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
		m = updated.(Model)
		if selectCmd == nil {
			t.Fatal("Enter did not select the media file")
		}
		updated, loadCmd := m.Update(selectCmd())
		m = updated.(Model)
		if loadCmd == nil {
			t.Fatal("selection did not schedule playback")
		}
		updated, _ = m.Update(loadCmd())
		m = updated.(Model)
		deadline := time.NewTimer(5 * time.Second)
		started := false
		for !started {
			select {
			case state := <-p.Events():
				updated, _ := m.Update(playerStateMsg(state))
				m = updated.(Model)
				started = state.Path == path && state.HasTime && state.Time > 0.1
			case <-deadline.C:
				t.Fatalf("Enter attempt %d did not produce advancing playback", attempt+1)
			case <-ctx.Done():
				deadline.Stop()
				t.Fatal(ctx.Err())
			}
		}
		deadline.Stop()
		if m.pending != path {
			t.Fatalf("Enter attempt %d lost current item: pending=%q", attempt+1, m.pending)
		}
	}
}

func TestPlaybackControlsAndObservedStatus(t *testing.T) {
	p := &fakePlayer{}
	m := New(t.TempDir(), "", p)
	m.pending, m.playing = "/tmp/movie.mp4", "/tmp/movie.mp4"
	m.status.Volume = 45
	for _, key := range []tea.Key{{Code: tea.KeySpace}, {Code: tea.KeyRight}, {Code: '+', Text: "+"}, {Code: 'f', Text: "f"}} {
		updated, cmd := m.Update(tea.KeyPressMsg(key))
		m = updated.(Model)
		if cmd == nil {
			t.Fatalf("key %q did not schedule a control", key.String())
		}
		_, _ = m.Update(cmd())
	}
	if len(p.paused) != 1 || !p.paused[0] || len(p.seeks) != 1 || p.seeks[0] != 5 || len(p.volumes) != 1 || p.volumes[0] != 50 || p.fullscreens != 1 {
		t.Fatalf("controls: pause=%v seek=%v volume=%v fullscreen=%d", p.paused, p.seeks, p.volumes, p.fullscreens)
	}
	m.status.Volume = 0
	for _, key := range []tea.Key{{Code: tea.KeyLeft}, {Code: '-', Text: "-"}, {Code: 's', Text: "s"}} {
		updated, cmd := m.Update(tea.KeyPressMsg(key))
		m = updated.(Model)
		if cmd == nil {
			t.Fatalf("key %q did not schedule a control", key.String())
		}
		_, _ = m.Update(cmd())
	}
	if len(p.seeks) != 2 || p.seeks[1] != -5 || len(p.volumes) != 2 || p.volumes[1] != 0 || p.stops != 1 {
		t.Fatalf("reverse/boundary controls: seek=%v volume=%v stop=%d", p.seeks, p.volumes, p.stops)
	}
	updated, _ := m.Update(playerStateMsg(playback.State{Path: m.playing, Paused: true, Time: 65, Duration: 120, Volume: 50, HasTime: true, HasDuration: true}))
	m = updated.(Model)
	view := m.View().Content
	if !strings.Contains(view, "Paused") || !strings.Contains(view, "1:05/2:00") || !strings.Contains(view, "Volume: 50%") {
		t.Fatalf("status missing from view: %s", view)
	}
	if m.status.HasDuration == false {
		t.Fatal("known duration was lost")
	}
}

func TestFullscreenDisabledForAudio(t *testing.T) {
	p := &fakePlayer{}
	m := New(t.TempDir(), "", p)
	m.playing = "/tmp/song.mp3"
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'f', Text: "f"}))
	if cmd != nil || updated.(Model).player == nil || p.fullscreens != 0 {
		t.Fatal("fullscreen should be unavailable for audio")
	}
}

func TestEndOfFileStatusAndFullscreenControls(t *testing.T) {
	p := &fakePlayer{}
	path := "/tmp/movie.mp4"
	m := New(t.TempDir(), "", p)
	m.pending, m.playing = path, path
	updated, _ := m.Update(playerStateMsg(playback.State{Path: path, Ended: true, EndReason: "eof"}))
	m = updated.(Model)
	if !strings.Contains(m.View().Content, "Ended") {
		t.Fatalf("ended status missing: %s", m.View().Content)
	}
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'f', Text: "f"}))
	if cmd != nil || p.fullscreens != 0 {
		t.Fatal("fullscreen remained active after EOF")
	}
	_ = updated
}

func TestEndOfFileKeepsStatusWhenMPVClearsPath(t *testing.T) {
	player := &fakePlayer{}
	path := "/tmp/movie.mp4"
	m := New(t.TempDir(), "", player)
	m.pending, m.playing = path, path
	updated, _ := m.Update(playerStateMsg(playback.State{Path: path, Ended: true, EndReason: "eof"}))
	m = updated.(Model)
	updated, _ = m.Update(playerStateMsg(playback.State{Ended: true, EndReason: "eof"}))
	m = updated.(Model)
	if m.pending != path || !strings.Contains(m.View().Content, "Now playing: "+path+"  Ended") {
		t.Fatalf("EOF status was lost when mpv cleared path: pending=%q view=%s", m.pending, m.View().Content)
	}
}

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
