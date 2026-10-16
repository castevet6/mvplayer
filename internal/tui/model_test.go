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
	"github.com/charmbracelet/x/ansi"
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
	if m.queueCursor < m.queueOffset || m.queueCursor >= m.queueOffset+max(1, m.height-9) {
		t.Fatalf("queue selection is outside narrow viewport: cursor=%d offset=%d", m.queueCursor, m.queueOffset)
	}
}
