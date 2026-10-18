package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"mvplayer/internal/browser"
	"mvplayer/internal/mediainfo"
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

func (p *fakePlayer) Load(_ context.Context, _ uint64, path string) error {
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
	m.status = playback.State{Ended: true, Ready: true, EndReason: "error", HasTime: true, HasDuration: true}
	updated, cmd := m.Update(FileSelectedMsg{Path: "/tmp/clip.mp4"})
	if cmd == nil {
		t.Fatal("selection did not schedule playback")
	}
	selected := updated.(Model)
	if selected.status.Ended || selected.status.Ready || selected.status.HasTime || selected.status.HasDuration {
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

func TestPlaybackAttemptRejectsStaleLoadAndPlayerEvents(t *testing.T) {
	p := &fakePlayer{}
	m := New(t.TempDir(), "", p)
	updated, oldCmd := m.Update(FileSelectedMsg{Path: "/tmp/same.mp4"})
	m = updated.(Model)
	oldAttempt := m.attempt
	updated, _ = m.Update(FileSelectedMsg{Path: "/tmp/same.mp4"})
	m = updated.(Model)
	newAttempt := m.attempt
	if newAttempt == oldAttempt {
		t.Fatal("repeated selection reused its attempt identity")
	}
	updated, _ = m.Update(loadResult{path: "/tmp/same.mp4", attempt: oldAttempt})
	m = updated.(Model)
	if m.playing != "" || !m.awaitingPlayback {
		t.Fatalf("stale load changed active attempt: %#v", m)
	}
	updated, _ = m.Update(playerStateMsg(playback.State{Attempt: oldAttempt, Path: "/tmp/same.mp4", Ready: true}))
	m = updated.(Model)
	if m.playing != "" || !m.awaitingPlayback {
		t.Fatalf("stale player event changed active attempt: %#v", m)
	}
	_ = oldCmd
}

func TestPlayerFailurePreservesBrowserAndQueue(t *testing.T) {
	p := &fakePlayer{}
	m := New(t.TempDir(), "", p)
	m.queue = []string{"/q/one.mp4", "/q/two.mp3"}
	m.queueIndex, m.queueResumeIndex = 0, 1
	m.cursor = 3
	updated, _ := m.Update(playerStateMsg(playback.State{Attempt: 0, Path: "/q/one.mp4", Ended: true, EndReason: "error", Failure: errors.New("mpv exited")}))
	m = updated.(Model)
	if m.playbackErr == nil || m.playing != "" || len(m.queue) != 2 || m.queueIndex != 0 || m.cursor != 3 {
		t.Fatalf("failure did not preserve usable application state: %#v", m)
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
	logPath := filepath.Join(t.TempDir(), "mpv.log")
	args = append(args, "--log-file="+logPath)
	p := playback.NewMPV(args...)
	defer func() {
		p.Close()
		if t.Failed() {
			if log, err := os.ReadFile(logPath); err == nil {
				var lines []string
				for _, line := range strings.Split(string(log), "\n") {
					if strings.Contains(line, "][e]") || strings.Contains(line, "][f]") || strings.Contains(line, "[file]") || strings.Contains(line, "Run command: loadfile") || strings.Contains(line, "EOF") {
						lines = append(lines, line)
					}
				}
				t.Logf("mpv failure details:\n%s", strings.Join(lines, "\n"))
			}
		}
	}()
	m := New(t.TempDir(), "", p)
	m.visible = []browser.Entry{{Name: filepath.Base(path), Path: path, Media: true}}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for attempt := 0; attempt < 6; attempt++ {
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
		if m.err != nil {
			t.Fatalf("Enter attempt %d: %v", attempt+1, m.err)
		}
		deadline := time.NewTimer(5 * time.Second)
		started := false
		for !started {
			select {
			case state := <-p.Events():
				updated, _ := m.Update(playerStateMsg(state))
				m = updated.(Model)
				if state.Ended && state.EndReason == "error" {
					t.Fatalf("Enter attempt %d: mpv failed playback", attempt+1)
				}
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

func TestMixedMediaQueueIntegration(t *testing.T) {
	if os.Getenv("MVPLAYER_MPV_QUEUE_INTEGRATION") != "1" {
		t.Skip("set MVPLAYER_MPV_QUEUE_INTEGRATION=1 to run real mpv queue checks")
	}
	video := filepath.Join("..", "..", "testdata", "mvplayer-check-video.mp4")
	audio := filepath.Join("..", "..", "testdata", "mvplayer-check-audio.wav")
	coverAudio := filepath.Join("..", "..", "testdata", "mvplayer-check-cover.mp3")
	sampleAudio := filepath.Join("..", "..", "testdata", "mvplayer-sample-audio.wav")
	sequences := [][]string{{video, audio, video}, {audio, video, audio}, {audio, coverAudio, sampleAudio}}
	for _, sequence := range sequences {
		p := playback.NewMPV("--vo=null", "--ao=null", "--end=0.5")
		m := New(t.TempDir(), "", p)
		m.queue = append([]string(nil), sequence...)
		m.queueCursor = 0
		messages := make(chan tea.Msg, 32)
		stop := make(chan struct{})
		var dispatch func(tea.Cmd)
		dispatch = func(cmd tea.Cmd) {
			if cmd == nil {
				return
			}
			go func() {
				msg := cmd()
				if batch, ok := msg.(tea.BatchMsg); ok {
					for _, child := range batch {
						dispatch(child)
					}
					return
				}
				if msg == nil {
					return
				}
				select {
				case messages <- msg:
				case <-stop:
				}
			}()
		}
		dispatch(m.listenPlayer())
		updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
		m = updated.(Model)
		dispatch(cmd)
		var readyPaths []string
		deadline := time.NewTimer(20 * time.Second)
		finished := false
		for !finished {
			select {
			case msg := <-messages:
				state, isPlayerState := msg.(playerStateMsg)
				updated, next := m.Update(msg)
				m = updated.(Model)
				if m.err != nil {
					_ = p.Close()
					t.Fatalf("queue load failed: %v", m.err)
				}
				if isPlayerState && state.Ready && state.Path != "" && (len(readyPaths) == 0 || readyPaths[len(readyPaths)-1] != state.Path) {
					if m.awaitingPlayback || !strings.Contains(m.View().Content, "Now playing: "+state.Path+"  Playing") {
						_ = p.Close()
						t.Fatalf("ready item still displayed as Loading: path=%q awaiting=%t view=%s", state.Path, m.awaitingPlayback, m.View().Content)
					}
					readyPaths = append(readyPaths, state.Path)
				}
				finished = len(readyPaths) == len(sequence) && m.queueIndex == len(sequence)-1 && !m.awaitingPlayback && m.status.Ended && m.status.EndReason == "eof"
				if !finished {
					dispatch(next)
				}
			case <-deadline.C:
				close(stop)
				_ = p.Close()
				t.Fatalf("queue did not finish: paths=%v index=%d status=%#v error=%v", readyPaths, m.queueIndex, m.status, m.err)
			}
		}
		deadline.Stop()
		close(stop)
		if err := p.Close(); err != nil {
			t.Fatalf("close mpv: %v", err)
		}
		if len(readyPaths) != len(sequence) {
			t.Fatalf("expected %d loaded queue items in %v, got %v", len(sequence), sequence, readyPaths)
		}
		for i := range sequence {
			want, _ := filepath.Abs(sequence[i])
			if readyPaths[i] != want {
				t.Fatalf("queue order: got %v, want %v", readyPaths, sequence)
			}
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

func TestMixedQueueControlsAndNaturalEOF(t *testing.T) {
	p := &fakePlayer{}
	first, second := "/media/first.mp4", "/media/second.mp3"
	m := New(t.TempDir(), "", p)
	m.queue = []string{first, second}
	m.queueCursor = 0
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("next did not start first queued item")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.queueIndex != 0 || m.pending != first || len(m.queue) != 2 {
		t.Fatalf("first queued load: index=%d pending=%q queue=%v", m.queueIndex, m.pending, m.queue)
	}
	updated, _ = m.Update(playerStateMsg(playback.State{Path: first, HasTime: true, Time: 0.1}))
	m = updated.(Model)
	_ = p.Events()
	p.events <- playback.State{Path: second, Ready: true}
	updated, cmd = m.Update(playerStateMsg(playback.State{Path: first, Ended: true, EndReason: "eof"}))
	m = updated.(Model)
	if cmd == nil || m.queueIndex != 1 || m.pending != second {
		t.Fatalf("EOF did not advance: index=%d pending=%q cmd=%v", m.queueIndex, m.pending, cmd != nil)
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("EOF transition did not batch load and listener: %T", cmd())
	}
	for _, child := range batch {
		msg := child()
		if result, ok := msg.(loadResult); ok {
			updated, _ = m.Update(result)
			m = updated.(Model)
		} else if state, ok := msg.(playerStateMsg); ok {
			updated, _ = m.Update(state)
			m = updated.(Model)
		}
	}
	if len(p.paths) != 2 || p.paths[1] != second {
		t.Fatalf("loaded paths = %v", p.paths)
	}
	// A delayed duplicate EOF from the prior item must not skip the new one.
	updated, cmd = m.Update(playerStateMsg(playback.State{Ended: true, EndReason: "eof"}))
	m = updated.(Model)
	if m.queueIndex != 1 || m.pending != second {
		t.Fatal("duplicate/stale EOF advanced queue")
	}
	// A failed subsequent item remains in the queue and does not trigger an
	// automatic attempt at another item.
	failed := "/media/failed.wav"
	m.queue = append(m.queue, failed)
	p.err = errors.New("load failed")
	updated, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("next did not attempt the queued item")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.playbackErr == nil || len(m.queue) != 3 || m.queueIndex != 2 {
		t.Fatalf("failed next did not preserve queue: error=%v index=%d queue=%v", m.playbackErr, m.queueIndex, m.queue)
	}
}

func TestImmediatePlayPreservesQueueAndEnqueueRemoval(t *testing.T) {
	p := &fakePlayer{}
	root := t.TempDir()
	queued := filepath.Join(root, "queued.mp3")
	chosen := filepath.Join(root, "chosen.mp4")
	m := NewWithLister(root, func(string) ([]browser.Entry, error) { return nil, nil })
	m.player = p
	m.queue = []string{queued}
	m.queueCursor = 0
	m.visible = []browser.Entry{{Name: "chosen.mp4", Path: chosen, Media: true}}
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"}))
	m = updated.(Model)
	if cmd != nil || len(m.queue) != 2 || m.queue[1] != chosen {
		t.Fatalf("enqueue failed: %v", m.queue)
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(Model)
	updated, cmd = m.Update(FileSelectedMsg{Path: chosen})
	m = updated.(Model)
	if cmd == nil || len(m.queue) != 2 || m.queueIndex != -1 {
		t.Fatalf("immediate play changed queue: queue=%v index=%d", m.queue, m.queueIndex)
	}
	// Move to and remove the second pending entry.
	m.focus = queuePane
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: ']', Text: "]"}))
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Text: "d"}))
	m = updated.(Model)
	if len(m.queue) != 1 || m.queue[0] != queued {
		t.Fatalf("pending removal failed: %v", m.queue)
	}
}

func TestQueueBoundariesPreviousAndImmediateResume(t *testing.T) {
	p := &fakePlayer{}
	paths := []string{"/q/a.mp4", "/q/b.mp3", "/q/c.mp4"}
	m := New(t.TempDir(), "", p)
	m.queue = append([]string(nil), paths...)
	m.queueCursor = 0
	// Start the first item, then previous at its boundary is a no-op.
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	updated, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: 'p', Text: "p"}))
	m = updated.(Model)
	if cmd != nil || len(p.paths) != 1 {
		t.Fatal("previous at queue start should be a no-op")
	}
	// Finish the first item, then immediately play another file.
	updated, _ = m.Update(playerStateMsg(playback.State{Path: paths[0], HasTime: true, Time: 1}))
	m = updated.(Model)
	updated, cmd = m.Update(playerStateMsg(playback.State{Path: paths[0], Ended: true, EndReason: "eof"}))
	m = updated.(Model)
	if cmd == nil || m.queueIndex != 1 {
		t.Fatal("first EOF did not select second queue item")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	updated, _ = m.Update(FileSelectedMsg{Path: "/q/instant.wav"})
	m = updated.(Model)
	updated, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	m = updated.(Model)
	if cmd == nil || m.queueIndex != 2 || m.pending != paths[2] {
		t.Fatalf("next after immediate play did not resume pending entry: index=%d pending=%q", m.queueIndex, m.pending)
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	// Exhaustion leaves queue state intact; previous replays the current queue item.
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	m = updated.(Model)
	if m.queueIndex != 2 || len(m.queue) != 3 {
		t.Fatal("next at exhaustion changed queue state")
	}
	updated, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: 'p', Text: "p"}))
	m = updated.(Model)
	if cmd == nil || m.queueIndex != 1 || m.pending != paths[1] {
		t.Fatal("previous did not load prior queue item")
	}
}

func TestQueuedItemReadinessWithoutTimePositionAndNaturalEOF(t *testing.T) {
	p := &fakePlayer{}
	first, second := "/q/short-audio.mp3", "/q/next.mp4"
	m := New(t.TempDir(), "", p)
	m.queue = []string{first, second}
	m.queueCursor = 0
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	// mpv can signal file-loaded before it has a useful time-pos snapshot.
	updated, _ = m.Update(playerStateMsg(playback.State{Path: first, Ready: true}))
	m = updated.(Model)
	if m.awaitingPlayback || !strings.Contains(m.View().Content, "Status: Playing") || !strings.Contains(m.View().Content, "Media: "+first) {
		t.Fatalf("file-loaded did not end Loading without time-pos: awaiting=%t view=%s", m.awaitingPlayback, m.View().Content)
	}
	updated, cmd = m.Update(playerStateMsg(playback.State{Path: first, Ended: true, EndReason: "eof"}))
	m = updated.(Model)
	if cmd == nil || m.queueIndex != 1 || m.pending != second {
		t.Fatalf("audio EOF did not advance: index=%d pending=%q cmd=%v", m.queueIndex, m.pending, cmd != nil)
	}
}

func TestQueueEOFAdvanceKeepsListeningForNextItem(t *testing.T) {
	p := &fakePlayer{events: make(chan playback.State, 1)}
	first, second := "/q/first.mp4", "/q/second.mp3"
	p.events <- playback.State{Path: second, Ready: true}
	m := New(t.TempDir(), "", p)
	m.queue = []string{first, second}
	m.queueIndex = 0
	m.queueResumeIndex = 1
	m.pending, m.playing = first, first
	m.status = playback.State{Path: first, Ready: true}
	updated, cmd := m.Update(playerStateMsg(playback.State{Path: first, Ended: true, EndReason: "eof"}))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("natural EOF did not schedule queue advancement")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("queue advancement must load the next item and keep listening; got %T (%#v)", cmd(), cmd())
	}
	var sawReady bool
	for _, command := range batch {
		msg := command()
		if ready, ok := msg.(playerStateMsg); ok {
			updated, _ := m.Update(ready)
			m = updated.(Model)
			sawReady = true
		}
	}
	if !sawReady || m.awaitingPlayback || m.pending != second {
		t.Fatalf("next item's player state was not consumed: awaiting=%t pending=%q", m.awaitingPlayback, m.pending)
	}
}

func TestQueuedFailureDoesNotAutoAdvance(t *testing.T) {
	p := &fakePlayer{}
	first, second := "/q/broken.wav", "/q/valid.mp3"
	m := New(t.TempDir(), "", p)
	m.queue = []string{first, second}
	m.queueCursor = 0
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	updated, _ = m.Update(playerStateMsg(playback.State{Path: first, Ready: true}))
	m = updated.(Model)
	updated, cmd = m.Update(playerStateMsg(playback.State{Path: first, Ended: true, EndReason: "error"}))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("failure event did not continue listening for player state")
	}
	if len(p.paths) != 1 || m.queueIndex != 0 || m.queue[1] != second {
		t.Fatalf("failed item advanced queue: paths=%v index=%d queue=%v", p.paths, m.queueIndex, m.queue)
	}
}

func TestQueueMarkersUseASCIIAndDistinguishSelection(t *testing.T) {
	m := NewWithLister(t.TempDir(), func(string) ([]browser.Entry, error) { return nil, nil })
	m.queue = []string{"/q/selected.mp3", "/q/playing.mp4"}
	m.queueCursor, m.queueIndex = 0, 1
	m.focus = queuePane
	view := m.View().Content
	if !strings.Contains(view, "> selected.mp3") || !strings.Contains(view, "* playing.mp4") || strings.Contains(view, "▶") {
		t.Fatalf("queue markers are not terminal-safe/distinct: %s", view)
	}
	m.queueCursor = 1
	view = m.View().Content
	if !strings.Contains(view, ">*playing.mp4") && !strings.Contains(view, ">* playing.mp4") {
		t.Fatalf("combined selection/playback marker missing: %s", view)
	}
}

func TestFocusPresentationAcrossResponsiveLayouts(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	m := NewWithLister("/media", func(string) ([]browser.Entry, error) { return nil, nil })
	m.colorProfile = colorprofile.ANSI
	m.visible = []browser.Entry{{Name: "browser.mp4", Path: "/media/browser.mp4", Media: true}}
	m.queue = []string{"/queue/queue.mp3"}
	m.cursor, m.queueCursor = 0, 0
	for _, width := range []int{24, 99, 100} {
		m.width = width
		m.focus = browserPane
		view := m.View().Content
		if width < 100 {
			if !strings.Contains(view, "[Browser]") || !strings.Contains(view, "Queue") || !strings.Contains(view, "Tab switch") || !strings.Contains(view, "\x1b[1;97;44m[Browser]\x1b[0m") {
				t.Fatalf("%d-column browser focus header: %q", width, view)
			}
		} else if !strings.Contains(view, "\x1b[1;97;44m[Browser]") || !strings.Contains(view, "\x1b[0m | ") || !strings.Contains(view, "> queue.mp3") {
			t.Fatalf("%d-column wide focus/retained selection: %q", width, view)
		}
		if width == 99 && !strings.Contains(view, "\x1b[1;4m> browser.mp4\x1b[0m") {
			t.Fatalf("active narrow row was not styled: %q", view)
		}
	}
	m.width, m.focus, m.queue = 40, queuePane, nil
	view := m.View().Content
	if !strings.Contains(view, "[Queue]") || !strings.Contains(view, "0/0") || !strings.Contains(view, "Queue is empty") {
		t.Fatalf("empty focused queue lost focus treatment: %q", view)
	}
}

func TestTabSwitchKeepsIndependentCursorsAtBothBreakpoints(t *testing.T) {
	m := NewWithLister("/media", func(string) ([]browser.Entry, error) { return nil, nil })
	m.visible = []browser.Entry{{Name: "a.mp4"}, {Name: "b.mp4"}}
	m.queue = []string{"/q/a.mp3", "/q/b.mp3", "/q/c.mp3"}
	m.cursor, m.queueCursor = 1, 2
	for _, width := range []int{99, 100} {
		m.width = width
		m.focus = browserPane
		updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
		m = updated.(Model)
		if m.focus != queuePane || m.cursor != 1 || m.queueCursor != 2 {
			t.Fatalf("%d-column Tab changed cursor state: focus=%d browser=%d queue=%d", width, m.focus, m.cursor, m.queueCursor)
		}
		if !strings.Contains(m.View().Content, "[Queue]") {
			t.Fatalf("%d-column Queue focus not visible", width)
		}
		updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
		m = updated.(Model)
		if m.focus != browserPane || m.cursor != 1 || m.queueCursor != 2 {
			t.Fatalf("%d-column reverse Tab changed cursor state: focus=%d browser=%d queue=%d", width, m.focus, m.cursor, m.queueCursor)
		}
	}
	m.queue = nil
	m.focus = browserPane
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	m = updated.(Model)
	if m.focus != queuePane || !strings.Contains(m.View().Content, "0/0") || !strings.Contains(m.View().Content, "Queue is empty") {
		t.Fatalf("Tab did not focus empty queue: focus=%d view=%s", m.focus, m.View().Content)
	}
}

func TestDeletionRequiresQueueFocus(t *testing.T) {
	m := NewWithLister("/media", func(string) ([]browser.Entry, error) { return nil, nil })
	m.player = &fakePlayer{}
	m.queue = []string{"/queue/one.mp3", "/queue/two.mp3"}
	m.queueCursor = 1
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Text: "d"}))
	m = updated.(Model)
	if len(m.queue) != 2 {
		t.Fatalf("browser focus removed queue entry: %v", m.queue)
	}
	m.focus = queuePane
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Text: "d"}))
	m = updated.(Model)
	if len(m.queue) != 1 || m.queue[0] != "/queue/one.mp3" {
		t.Fatalf("queue focus did not remove selected pending entry: %v", m.queue)
	}
}

func TestColorAndNoColorPresentation(t *testing.T) {
	for _, tc := range []struct {
		name, noColor string
		wantColor     bool
	}{
		{name: "unset", wantColor: true},
		{name: "empty", noColor: "", wantColor: true},
		{name: "zero", noColor: "0"},
		{name: "false", noColor: "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "unset" {
				t.Setenv("NO_COLOR", "")
				if err := os.Unsetenv("NO_COLOR"); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Setenv("NO_COLOR", tc.noColor)
			}
			m := NewWithLister("/media", func(string) ([]browser.Entry, error) { return nil, nil })
			m.colorProfile = colorprofile.ANSI
			m.visible = []browser.Entry{{Name: "clip.mp4", Path: "/media/clip.mp4"}}
			view := m.View().Content
			hasColor := strings.Contains(view, "\x1b[1;97;44m")
			if hasColor != tc.wantColor {
				t.Fatalf("color=%t, want %t: %q", hasColor, tc.wantColor, view)
			}
			if !tc.wantColor && !strings.Contains(view, "\x1b[1m") {
				t.Fatalf("NO_COLOR should retain bold text cues: %q", view)
			}
		})
	}
	t.Setenv("NO_COLOR", "")
	m := NewWithLister("/media", func(string) ([]browser.Entry, error) { return nil, nil })
	m.colorProfile = colorprofile.ASCII
	m.visible = []browser.Entry{{Name: "clip.mp4", Path: "/media/clip.mp4"}}
	if view := m.View().Content; strings.Contains(view, "\x1b[1;97;44m") || !strings.Contains(view, "\x1b[1m") {
		t.Fatalf("ASCII profile should use bold only: %q", view)
	}
	m.colorProfile = colorprofile.NoTTY
	if view := m.View().Content; strings.Contains(view, "\x1b[") {
		t.Fatalf("NoTTY profile should use plain text: %q", view)
	}
}

func TestFilterTabFocusAndANSIInputSanitization(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	m := NewWithLister("/media", func(string) ([]browser.Entry, error) { return nil, nil })
	m.colorProfile = colorprofile.ANSI
	m.visible = []browser.Entry{{Name: "clip\x1b[31m.mp4", Path: "/media/clip.mp4"}}
	m.filtering = true
	m.query = "x\x1b[2J"
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	m = updated.(Model)
	if m.filtering || m.focus != queuePane {
		t.Fatalf("Tab did not finish filtering and focus Queue: %#v", m)
	}
	m.focus = browserPane
	view := m.View().Content
	if strings.Contains(view, "\x1b[31m") || strings.Contains(view, "\x1b[2J") || !strings.Contains(view, "Filter: x [2J") {
		t.Fatalf("untrusted controls escaped into styled rendering: %q", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if ansi.StringWidth(line) > m.width {
			t.Fatalf("styled line exceeds width: %d > %d: %q", ansi.StringWidth(line), m.width, line)
		}
	}
}

func TestBrowserFooterKeepsHelpAndQuitVisible(t *testing.T) {
	for _, width := range []int{80, 100, 110} {
		t.Run(fmt.Sprintf("%d columns", width), func(t *testing.T) {
			m := NewWithLister("/media", func(string) ([]browser.Entry, error) { return nil, nil })
			m.width = width
			view := m.View().Content
			footer := view[strings.LastIndex(view, "\n")+1:]
			if !strings.Contains(footer, "? help") || !strings.Contains(footer, "q quit") {
				t.Fatalf("footer hid help or quit at width %d: %q", width, footer)
			}
			if strings.Contains(footer, "d remove") {
				t.Fatalf("browser footer advertised Queue-only deletion at width %d: %q", width, footer)
			}
			m.filtering = true
			view = m.View().Content
			footer = view[strings.LastIndex(view, "\n")+1:]
			if strings.Contains(footer, "? help") || !strings.Contains(footer, "Ctrl+C quit") {
				t.Fatalf("filter footer gave inaccurate help/quit hints at width %d: %q", width, footer)
			}
		})
	}
}

func TestControlCharactersAreSanitizedBeforeStyling(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	m := NewWithLister("/media\x1b[2J", func(string) ([]browser.Entry, error) { return nil, nil })
	m.colorProfile = colorprofile.ANSI
	m.browseDir = "/media\x1b[2J"
	m.visible = []browser.Entry{{Name: "clip\x1b[31m.mp4", Path: "/media/clip.mp4"}}
	m.err = errors.New("denied\x1b[2J")
	m.query, m.filtering = "x\x1b[31m", true
	view := m.View().Content
	if strings.Contains(view, "\x1b[2J") || strings.Contains(view, "\x1b[31m") {
		t.Fatalf("untrusted control sequence reached output: %q", view)
	}
	if !strings.Contains(view, "Filter (editing): x [31m") || !strings.Contains(view, "Browser error: denied [2J") || !strings.Contains(view, "/media [2J") {
		t.Fatalf("sanitized user text missing: %q", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if ansi.StringWidth(line) > m.width {
			t.Fatalf("ANSI-aware fitting failed: width=%d line=%q", ansi.StringWidth(line), line)
		}
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
	if m.pending != path || !strings.Contains(m.View().Content, "Status: Ended") || !strings.Contains(m.View().Content, "Media: "+path) {
		t.Fatalf("EOF status was lost when mpv cleared path: pending=%q view=%s", m.pending, m.View().Content)
	}
}

func TestQuitEndFileRendersStopped(t *testing.T) {
	if got := playbackEndLabel("quit"); got != "Stopped" {
		t.Fatalf("quit label = %q, want Stopped", got)
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

func TestInfoModalPreservesBrowserAndRejectsStaleResults(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "same name.mp4")
	if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	player := &fakePlayer{}
	m := New(root, "", player)
	m.loading = false
	m.focus = browserPane
	m.recursive = true
	m.query = "same"
	m.cursor = 0
	m.browserOffset = 3
	m.queue = []string{"/queued.mp3"}
	m.queueCursor = 0
	m.playing = "/playing.mp4"
	m.visible = []browser.Entry{{Name: "same name.mp4", Path: path, Media: true}}
	m.inspector = mediainfo.Inspector{LookPath: func(string) (string, error) { return "/ffprobe", nil }, Run: func(context.Context, string, ...string) ([]byte, []byte, error) {
		return []byte(`{"format":{"format_name":"mp4","duration":"2"},"streams":[]}`), nil, nil
	}}
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'i', Text: "i"}))
	m = updated.(Model)
	if !m.infoOpen || !m.infoLoading || cmd == nil || m.infoPath != path {
		t.Fatalf("modal did not open: %#v", m)
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.infoLoading || m.info.Format != "mp4" || m.playing != "/playing.mp4" || len(player.paths) != 0 {
		t.Fatalf("inspection affected playback or failed: modal=%#v paths=%v", m, player.paths)
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	m = updated.(Model)
	if !m.infoOpen || m.focus != browserPane || m.cursor != 0 || m.browserOffset != 3 || m.query != "same" || !m.recursive || m.queue[0] != "/queued.mp3" {
		t.Fatalf("modal changed browsing context: %#v", m)
	}
	old := inspectResult{request: m.infoRequest, info: mediainfo.Info{Format: "old"}}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	m = updated.(Model)
	if m.infoOpen {
		t.Fatal("Escape did not dismiss modal")
	}
	m.openInfo(filepath.Join(root, "new.mp4"))
	updated, _ = m.Update(old)
	m = updated.(Model)
	if m.info.Format == "old" {
		t.Fatal("late result from previous modal request was applied")
	}
}

func TestInfoShortcutUnavailableDuringScanAndFilter(t *testing.T) {
	m := NewWithLister("/media", func(string) ([]browser.Entry, error) { return nil, nil })
	m.visible = []browser.Entry{{Name: "a.mp4", Path: "/media/a.mp4", Media: true}}
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'i', Text: "i"}))
	m = updated.(Model)
	if m.infoOpen {
		t.Fatal("inspection opened while browser was loading")
	}
	m.loading = false
	m.filtering = true
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'i', Text: "i"}))
	m = updated.(Model)
	if m.infoOpen || m.query != "i" {
		t.Fatalf("filter did not accept i: open=%t query=%q", m.infoOpen, m.query)
	}
}

func TestInfoModalFitsSupportedTerminalSizes(t *testing.T) {
	for _, size := range []tea.WindowSizeMsg{{Width: 120, Height: 30}, {Width: 100, Height: 24}, {Width: 80, Height: 24}, {Width: 40, Height: 12}, {Width: 24, Height: 10}} {
		m := NewWithLister("/media", func(string) ([]browser.Entry, error) { return nil, nil })
		m.width, m.height, m.infoOpen, m.infoPath = size.Width, size.Height, true, "/media/長い名前-clip.mp4"
		m.info = mediainfo.Info{Path: m.infoPath, Name: "長い名前-clip.mp4", Size: 1234567, Modified: time.Now(), Format: "matroska,webm", Duration: "01:02:03.456", Bitrate: "2 Mbit/s", Warning: strings.Repeat("probe warning ", 12)}
		m.info.Video = []mediainfo.Stream{{Index: 0, Codec: "h264", Width: 1920, Height: 1080, Rate: "29.970 fps average", Bitrate: "1 Mbit/s", Language: "eng", Title: "main"}}
		view := strings.Split(m.View().Content, "\n")
		if len(view) > size.Height {
			t.Fatalf("%dx%d modal has %d rows", size.Width, size.Height, len(view))
		}
		for i, line := range view {
			if width := ansi.StringWidth(line); width > size.Width {
				t.Fatalf("%dx%d line %d has width %d: %q", size.Width, size.Height, i, width, line)
			}
		}
	}
}

func TestRecursiveToggleRetainsFilterAndSelection(t *testing.T) {
	root := t.TempDir()
	mediaPath := filepath.Join(root, "album", "live", "song.flac")
	if err := os.MkdirAll(filepath.Dir(mediaPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mediaPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	m := NewWithLister(root, browser.List)
	m.playing = "/active/now.mp4"
	m.pending = m.playing
	m.queue = []string{"/queue/pending.mp3"}
	m.visible = []browser.Entry{{Name: "song.flac", Path: mediaPath, Media: true}}
	m.query = "live"
	m.cursor = 0
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'r', Text: "r"}))
	m = updated.(Model)
	if !m.recursive || !m.loading || m.query != "live" || cmd == nil {
		t.Fatalf("recursive toggle state: recursive=%t loading=%t query=%q cmd=%t", m.recursive, m.loading, m.query, cmd != nil)
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.loading || len(m.visible) != 1 || m.visible[0].Path != mediaPath || m.cursor != 0 {
		t.Fatalf("recursive result did not retain path selection: loading=%t cursor=%d entries=%#v", m.loading, m.cursor, m.visible)
	}
	if m.playing != "/active/now.mp4" || len(m.queue) != 1 || m.queue[0] != "/queue/pending.mp3" {
		t.Fatalf("recursive toggle changed playback or queue: playing=%q queue=%v", m.playing, m.queue)
	}
	view := m.View().Content
	if !strings.Contains(view, "Recursive") || !strings.Contains(view, "album/live/song.flac") {
		t.Fatalf("recursive view missing mode or relative path: %q", view)
	}
}

func TestDirectoryResultKeepsRestoredSelectionVisible(t *testing.T) {
	root := t.TempDir()
	m := NewWithLister(root, func(string) ([]browser.Entry, error) { return nil, nil })
	m.width, m.height = 80, 12
	for i := 0; i < 30; i++ {
		path := filepath.Join(root, fmt.Sprintf("old-%02d.mp3", i))
		m.visible = append(m.visible, browser.Entry{Name: filepath.Base(path), Path: path, Media: true})
	}
	m.cursor, m.browserOffset = 25, 20
	selectedPath := m.visible[m.cursor].Path
	m.startDirectoryScan(true, true)
	entries := make([]browser.Entry, 12)
	for i := range entries {
		path := filepath.Join(root, fmt.Sprintf("new-%02d.mp3", i))
		entries[i] = browser.Entry{Name: filepath.Base(path), Path: path, RelativePath: filepath.Base(path), Media: true}
	}
	entries[10] = browser.Entry{Name: "restored.mp3", Path: selectedPath, RelativePath: "restored.mp3", Media: true}
	updated, _ := m.Update(directoryResult{request: m.request, path: root, recursive: true, entries: entries})
	m = updated.(Model)
	if m.cursor != 10 || m.cursor < m.browserOffset || m.cursor >= m.browserOffset+m.listRows() {
		t.Fatalf("restored selection is outside viewport: cursor=%d offset=%d rows=%d", m.cursor, m.browserOffset, m.listRows())
	}
	if !strings.Contains(m.View().Content, "> restored.mp3") {
		t.Fatalf("restored selection is not rendered: %s", m.View().Content)
	}
}

func TestNarrowQueueScrollUsesRenderedRowCount(t *testing.T) {
	m := NewWithLister(t.TempDir(), func(string) ([]browser.Entry, error) { return nil, nil })
	m.width, m.height, m.focus = 80, 12, queuePane
	for i := 0; i < 10; i++ {
		m.queue = append(m.queue, fmt.Sprintf("/queue/%d.mp3", i))
	}
	m.queueCursor = 3
	m.keepVisible()
	if m.queueCursor < m.queueOffset || m.queueCursor >= m.queueOffset+m.listRows() {
		t.Fatalf("queue selection is outside viewport: cursor=%d offset=%d rows=%d", m.queueCursor, m.queueOffset, m.listRows())
	}
	if !strings.Contains(m.View().Content, "> 3.mp3") {
		t.Fatalf("selected queue row is not rendered: %s", m.View().Content)
	}
}

func TestRecursiveWarningIsVisibleAndStaleWarningsAreIgnored(t *testing.T) {
	root := t.TempDir()
	m := NewWithLister(root, browser.List)
	m.recursive = true
	m.loading = true
	m.request++
	warning := "could not read failed; skipped 1 subtree(s)"
	current := directoryResult{request: m.request, path: root, recursive: true, warning: warning}
	updated, _ := m.Update(current)
	m = updated.(Model)
	if m.warning != warning || !strings.Contains(m.View().Content, "Partial results: "+warning) {
		t.Fatalf("recursive warning missing from view: %q", m.View().Content)
	}
	stale := directoryResult{request: m.request - 1, path: root, recursive: true, warning: "stale warning"}
	updated, _ = m.Update(stale)
	if got := updated.(Model).warning; got != warning {
		t.Fatalf("stale warning replaced current warning: %q", got)
	}
}

func TestChangingModeCancelsObsoleteRecursiveScan(t *testing.T) {
	root := t.TempDir()
	canceled := make(chan struct{})
	m := NewWithLister(root, browser.List)
	m.recursive = true
	m.listRecursive = func(ctx context.Context, _ string) ([]browser.Entry, string, error) {
		<-ctx.Done()
		close(canceled)
		return nil, "", ctx.Err()
	}
	oldScan := m.loadDirectory()
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'r', Text: "r"}))
	m = updated.(Model)
	if m.recursive {
		t.Fatal("mode toggle did not return to Directory mode")
	}
	oldResult := oldScan()
	select {
	case <-canceled:
	default:
		t.Fatal("obsolete recursive scan was not canceled")
	}
	updated, _ = m.Update(oldResult)
	if updated.(Model).loading != true || updated.(Model).recursive {
		t.Fatalf("obsolete result changed current scan state: %#v", updated.(Model))
	}
}

func TestRecursiveMediaActionsUseAbsoluteEntryPath(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "one", "song.flac")
	second := filepath.Join(root, "two", "song.flac")
	player := &fakePlayer{}
	m := NewWithLister(root, browser.List)
	m.player = player
	m.recursive = true
	m.entries = []browser.Entry{
		{Name: "one/song.flac", Path: first, RelativePath: "one/song.flac", Media: true},
		{Name: "two/song.flac", Path: second, RelativePath: "two/song.flac", Media: true},
	}
	m.applyFilter()
	m.cursor = 1
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"}))
	m = updated.(Model)
	if len(m.queue) != 1 || m.queue[0] != second {
		t.Fatalf("enqueued path = %v, want %q", m.queue, second)
	}
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("Enter did not select recursive media")
	}
	selected, ok := cmd().(FileSelectedMsg)
	if !ok || selected.Path != second {
		t.Fatalf("selected recursive media = %#v", selected)
	}
	if got := updated.(Model).selected; got != second {
		t.Fatalf("model selected %q, want absolute path %q", got, second)
	}
}

func TestRecursiveToggleKeyContextAndEmptySelection(t *testing.T) {
	m := NewWithLister(t.TempDir(), func(string) ([]browser.Entry, error) { return nil, nil })
	m.filtering = true
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'r', Text: "r"}))
	m = updated.(Model)
	if m.recursive || m.query != "r" {
		t.Fatalf("r while filtering: recursive=%t query=%q", m.recursive, m.query)
	}
	m.filtering = false
	m.focus = queuePane
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'r', Text: "r"}))
	m = updated.(Model)
	if m.recursive {
		t.Fatal("r toggled recursive mode while Queue had focus")
	}
	m.focus = browserPane
	m.recursive = true
	m.visible = nil
	m.cursor = 0
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd != nil || updated.(Model).selected != "" {
		t.Fatalf("empty recursive result remained actionable: selected=%q cmd=%t", updated.(Model).selected, cmd != nil)
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

func TestPaneFocusQueueEnterAndResize(t *testing.T) {
	p := &fakePlayer{}
	m := NewWithLister("/media", func(string) ([]browser.Entry, error) { return nil, nil })
	m.player = p
	m.visible = []browser.Entry{{Name: "a.mp4", Path: "/media/a.mp4", Media: true}, {Name: "b.mp4", Path: "/media/b.mp4", Media: true}}
	m.queue = []string{"/queue/dup.mp3", "/queue/dup.mp3"}
	m.queueCursor = 1
	m.cursor = 0
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m = updated.(Model)
	if m.focus != queuePane || m.cursor != 0 || m.queueCursor != 1 {
		t.Fatalf("focused movement changed wrong selection: focus=%d browser=%d queue=%d", m.focus, m.cursor, m.queueCursor)
	}
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(Model)
	if cmd == nil || m.queueIndex != 1 || m.pending != "/queue/dup.mp3" {
		t.Fatalf("Enter did not play selected duplicate: index=%d pending=%q", m.queueIndex, m.pending)
	}
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 99, Height: 24})
	m = updated.(Model)
	if !strings.Contains(m.View().Content, "Queue") || strings.Contains(m.View().Content, " | ") {
		t.Fatalf("narrow view did not show focused pane: %s", m.View().Content)
	}
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(Model)
	view := m.View().Content
	if !strings.Contains(view, " | ") || !strings.Contains(view, "dup.mp3") {
		t.Fatalf("wide view did not render both panes: %s", view)
	}
}

func TestFilterPrintableQuitAndIndependentErrors(t *testing.T) {
	m := NewWithLister("/media", func(string) ([]browser.Entry, error) { return nil, nil })
	m.entries = []browser.Entry{{Name: "question?q.mp4", Path: "/media/question?q.mp4", Media: true}}
	m.applyFilter()
	m.filtering = true
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Text: "q"}))
	m = updated.(Model)
	if m.quitting || m.query != "q" {
		t.Fatalf("q did not remain filter text: quitting=%t query=%q", m.quitting, m.query)
	}
	m.err, m.playbackErr = errors.New("directory denied"), errors.New("mpv disconnected")
	updated, _ = m.Update(directoryResult{request: m.request, path: m.browseDir, entries: nil})
	m = updated.(Model)
	if m.err != nil || m.playbackErr == nil {
		t.Fatalf("successful listing changed unrelated playback error: browser=%v playback=%v", m.err, m.playbackErr)
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: '?', Text: "?"}))
	m = updated.(Model)
	m.helpOffset = 100
	if !m.help || !strings.Contains(m.View().Content, "Playback error: mpv disconnected") {
		t.Fatalf("help did not expose current error: %s", m.View().Content)
	}
}

func TestResponsiveViewFitsFrameAndScrolls(t *testing.T) {
	m := NewWithLister("/media", func(string) ([]browser.Entry, error) { return nil, nil })
	for i := 0; i < 40; i++ {
		m.visible = append(m.visible, browser.Entry{Name: strings.Repeat("界", 24) + string(rune('a'+i%26)), Path: filepath.Join("/media", fmt.Sprint(i)), Media: true})
		m.queue = append(m.queue, filepath.Join("/queue", strings.Repeat("🎬", 16)+fmt.Sprint(i)+".mp4"))
	}
	m.cursor, m.queueCursor = 39, 39
	for _, size := range []tea.WindowSizeMsg{{Width: 120, Height: 30}, {Width: 100, Height: 24}, {Width: 99, Height: 24}, {Width: 80, Height: 24}, {Width: 40, Height: 12}, {Width: 23, Height: 9}} {
		updated, _ := m.Update(size)
		m = updated.(Model)
		lines := strings.Split(m.View().Content, "\n")
		if len(lines) > size.Height {
			t.Fatalf("%dx%d view has %d rows", size.Width, size.Height, len(lines))
		}
		for i, line := range lines {
			if ansi.StringWidth(line) > size.Width {
				t.Fatalf("%dx%d row %d width %d", size.Width, size.Height, i, ansi.StringWidth(line))
			}
		}
	}
	if m.width != 23 || m.height != 9 || m.browserOffset == 0 || m.queueOffset == 0 {
		t.Fatalf("resize/scroll state not retained: %#v", m)
	}
	m.focus = queuePane
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	m = updated.(Model)
	if m.queueCursor < m.queueOffset || m.queueCursor >= m.queueOffset+max(1, m.height-8) {
		t.Fatalf("queue selection is outside narrow viewport: cursor=%d offset=%d", m.queueCursor, m.queueOffset)
	}
}
