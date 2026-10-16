package playback

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

var errIPCDisconnected = errors.New("mpv IPC disconnected")

// Player is the small application-facing playback boundary.
type Player interface {
	Load(context.Context, string) error
	Pause(context.Context, bool) error
	Seek(context.Context, float64) error
	SetVolume(context.Context, float64) error
	Stop(context.Context) error
	Fullscreen(context.Context, bool) error
	Events() <-chan State
	Close() error
}

// State is a snapshot derived from mpv's observed properties.
type State struct {
	Path                 string
	Paused               bool
	Ended                bool
	EndReason            string
	Time, Duration       float64
	Volume               float64
	Fullscreen           bool
	HasTime, HasDuration bool
}

// MPV manages one mpv process and its private JSON IPC socket.
type MPV struct {
	startMu   sync.Mutex
	loadMu    sync.Mutex
	mu        sync.Mutex
	conn      net.Conn
	cmd       *exec.Cmd
	dir       string
	extraArgs []string
	writes    sync.Mutex
	next      atomic.Uint64
	pending   map[uint64]chan response
	closed    chan struct{}
	events    chan State
	state     State
}

type response struct {
	Error string          `json:"error"`
	Data  json.RawMessage `json:"data"`
}

func NewMPV(extraArgs ...string) *MPV {
	return &MPV{pending: make(map[uint64]chan response), closed: make(chan struct{}), events: make(chan State, 32), extraArgs: extraArgs}
}

func (p *MPV) start(ctx context.Context) error {
	p.startMu.Lock()
	defer p.startMu.Unlock()
	if p.sessionClosed() {
		if err := p.restartLocked(ctx); err != nil {
			return err
		}
		return nil
	}
	return p.startLocked(ctx)
}

func (p *MPV) startLocked(ctx context.Context) error {
	p.mu.Lock()
	if p.conn != nil {
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()
	bin, err := exec.LookPath("mpv")
	if err != nil {
		return fmt.Errorf("mpv is required for playback; install it with: sudo apt install mpv")
	}
	dir, err := os.MkdirTemp("", "mvplayer-mpv-")
	if err != nil {
		return fmt.Errorf("create private mpv socket directory: %w", err)
	}
	if err = os.Chmod(dir, 0700); err != nil {
		os.RemoveAll(dir)
		return fmt.Errorf("secure mpv socket directory: %w", err)
	}
	socket := filepath.Join(dir, "mpv.sock")
	args := []string{"--idle=yes", "--no-terminal", "--input-ipc-server=" + socket, "--audio-display=no"}
	args = append(args, p.extraArgs...)
	cmd := exec.Command(bin, args...)
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return fmt.Errorf("start mpv: %w", err)
	}
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	var conn net.Conn
	for conn == nil {
		c, e := net.DialTimeout("unix", socket, 100*time.Millisecond)
		if e == nil {
			conn = c
			break
		}
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			os.RemoveAll(dir)
			return fmt.Errorf("wait for mpv IPC: %w", ctx.Err())
		case <-deadline.C:
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			os.RemoveAll(dir)
			return fmt.Errorf("mpv IPC socket did not become ready within 3 seconds")
		case <-tick.C:
		}
	}
	p.mu.Lock()
	p.conn, p.cmd, p.dir = conn, cmd, dir
	p.mu.Unlock()
	go p.readLoop(conn)
	for _, property := range []string{"path", "pause", "time-pos", "duration", "volume", "fullscreen"} {
		observeCtx, cancel := context.WithTimeout(ctx, time.Second)
		_, _ = p.command(observeCtx, "observe_property", 1, property)
		cancel()
	}
	return nil
}

func (p *MPV) sessionClosed() bool {
	p.mu.Lock()
	done := p.closed
	conn := p.conn
	p.mu.Unlock()
	if conn == nil {
		return false
	}
	select {
	case <-done:
		return true
	default:
		return false
	}
}

// restartLocked discards a dead IPC session and creates a fresh managed mpv.
// startMu must be held by the caller.
func (p *MPV) restartLocked(ctx context.Context) error {
	p.mu.Lock()
	conn, cmd, dir, done := p.conn, p.cmd, p.dir, p.closed
	p.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
	if conn != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
	p.mu.Lock()
	p.conn, p.cmd, p.dir = nil, nil, ""
	p.closed = make(chan struct{})
	p.pending = make(map[uint64]chan response)
	p.mu.Unlock()
	return p.startLocked(ctx)
}

func (p *MPV) Events() <-chan State { return p.events }
func (p *MPV) Pause(ctx context.Context, value bool) error {
	_, err := p.command(ctx, "set_property", "pause", value)
	return err
}
func (p *MPV) Seek(ctx context.Context, seconds float64) error {
	_, err := p.command(ctx, "seek", seconds, "relative")
	return err
}
func (p *MPV) SetVolume(ctx context.Context, value float64) error {
	_, err := p.command(ctx, "set_property", "volume", value)
	return err
}
func (p *MPV) Stop(ctx context.Context) error { _, err := p.command(ctx, "stop"); return err }
func (p *MPV) Fullscreen(ctx context.Context, value bool) error {
	_, err := p.command(ctx, "set_property", "fullscreen", value)
	return err
}

func (p *MPV) Load(ctx context.Context, path string) error {
	p.loadMu.Lock()
	defer p.loadMu.Unlock()
	if err := p.start(ctx); err != nil {
		return err
	}
	_, err := p.command(ctx, "loadfile", path, "replace")
	if errors.Is(err, errIPCDisconnected) {
		p.startMu.Lock()
		restartErr := p.restartLocked(ctx)
		p.startMu.Unlock()
		if restartErr != nil {
			return fmt.Errorf("recover mpv IPC: %w", restartErr)
		}
		_, err = p.command(ctx, "loadfile", path, "replace")
	}
	if err != nil {
		return fmt.Errorf("load %q: %w", path, err)
	}
	return nil
}

func (p *MPV) command(ctx context.Context, name string, args ...any) (json.RawMessage, error) {
	id := p.next.Add(1)
	ch := make(chan response, 1)
	p.mu.Lock()
	p.pending[id] = ch
	conn := p.conn
	p.mu.Unlock()
	if conn == nil {
		return nil, errors.New("mpv IPC is not connected")
	}
	payload, _ := json.Marshal(map[string]any{"command": append([]any{name}, args...), "request_id": id})
	p.writes.Lock()
	_, err := fmt.Fprintf(conn, "%s\n", payload)
	p.writes.Unlock()
	if err != nil {
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		return nil, fmt.Errorf("%w: write mpv command: %v", errIPCDisconnected, err)
	}
	select {
	case r := <-ch:
		if r.Error != "success" {
			return nil, fmt.Errorf("mpv command %s: %s", name, r.Error)
		}
		return r.Data, nil
	case <-ctx.Done():
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		return nil, ctx.Err()
	case <-p.closed:
		select {
		case r := <-ch:
			if r.Error != "success" {
				return nil, fmt.Errorf("mpv command %s: %s", name, r.Error)
			}
			return r.Data, nil
		default:
			return nil, errIPCDisconnected
		}
	}
}

func (p *MPV) readLoop(conn net.Conn) {
	defer close(p.closed)
	s := bufio.NewScanner(conn)
	s.Buffer(make([]byte, 4096), 1<<20)
	for s.Scan() {
		var msg struct {
			ID     uint64          `json:"request_id"`
			Error  string          `json:"error"`
			Data   json.RawMessage `json:"data"`
			Event  string          `json:"event"`
			Name   string          `json:"name"`
			Reason string          `json:"reason"`
		}
		if json.Unmarshal(s.Bytes(), &msg) != nil {
			continue
		}
		if msg.Event == "property-change" {
			p.updateProperty(msg.Name, msg.Data)
			continue
		}
		switch msg.Event {
		case "start-file", "file-loaded":
			p.updatePlaybackEvent(false, "")
		case "end-file":
			p.updatePlaybackEvent(true, msg.Reason)
		}
		if msg.ID == 0 {
			continue
		}
		p.mu.Lock()
		ch := p.pending[msg.ID]
		delete(p.pending, msg.ID)
		p.mu.Unlock()
		if ch != nil {
			ch <- response{Error: msg.Error, Data: msg.Data}
		}
	}
	p.mu.Lock()
	for id, ch := range p.pending {
		delete(p.pending, id)
		ch <- response{Error: "IPC disconnected"}
	}
	p.mu.Unlock()
}

func (p *MPV) updateProperty(name string, raw json.RawMessage) {
	p.mu.Lock()
	switch name {
	case "path":
		if string(raw) == "null" {
			p.state.Path = ""
		} else {
			_ = json.Unmarshal(raw, &p.state.Path)
		}
	case "pause":
		_ = json.Unmarshal(raw, &p.state.Paused)
	case "time-pos":
		if string(raw) == "null" {
			p.state.HasTime = false
		} else {
			p.state.HasTime = json.Unmarshal(raw, &p.state.Time) == nil
		}
	case "duration":
		if string(raw) == "null" {
			p.state.HasDuration = false
		} else {
			p.state.HasDuration = json.Unmarshal(raw, &p.state.Duration) == nil
		}
	case "volume":
		_ = json.Unmarshal(raw, &p.state.Volume)
	case "fullscreen":
		_ = json.Unmarshal(raw, &p.state.Fullscreen)
	}
	state := p.state
	p.mu.Unlock()
	p.publishState(state)
}

func (p *MPV) updatePlaybackEvent(ended bool, reason string) {
	p.mu.Lock()
	p.state.Ended, p.state.EndReason = ended, reason
	state := p.state
	p.mu.Unlock()
	p.publishState(state)
}

func (p *MPV) publishState(state State) {
	select {
	case p.events <- state:
	default:
		select {
		case <-p.events:
		default:
		}
		select {
		case p.events <- state:
		default:
		}
	}
}

func (p *MPV) Close() error {
	p.startMu.Lock()
	defer p.startMu.Unlock()
	p.mu.Lock()
	cmd, conn, dir := p.cmd, p.conn, p.dir
	p.mu.Unlock()
	if cmd == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	_, _ = p.command(ctx, "quit")
	cancel()
	p.mu.Lock()
	p.conn, p.cmd, p.dir = nil, nil, ""
	p.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	_ = cmd.Wait()
	if dir != "" {
		return os.RemoveAll(dir)
	}
	return nil
}
