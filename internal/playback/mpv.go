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

// Player is the small application-facing playback boundary.
type Player interface {
	Load(context.Context, string) error
	Close() error
}

// MPV manages one mpv process and its private JSON IPC socket.
type MPV struct {
	startMu sync.Mutex
	mu      sync.Mutex
	conn    net.Conn
	cmd     *exec.Cmd
	dir     string
	writes  sync.Mutex
	next    atomic.Uint64
	pending map[uint64]chan response
	closed  chan struct{}
}

type response struct {
	Error string          `json:"error"`
	Data  json.RawMessage `json:"data"`
}

func NewMPV() *MPV { return &MPV{pending: make(map[uint64]chan response), closed: make(chan struct{})} }

func (p *MPV) start(ctx context.Context) error {
	p.startMu.Lock()
	defer p.startMu.Unlock()
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
	cmd := exec.Command(bin, "--idle=yes", "--no-terminal", "--input-ipc-server="+socket, "--audio-display=no")
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
	return nil
}

func (p *MPV) Load(ctx context.Context, path string) error {
	if err := p.start(ctx); err != nil {
		return err
	}
	_, err := p.command(ctx, "loadfile", path, "replace")
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
		return nil, fmt.Errorf("write mpv command: %w", err)
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
			return nil, errors.New("mpv IPC disconnected")
		}
	}
}

func (p *MPV) readLoop(conn net.Conn) {
	defer close(p.closed)
	s := bufio.NewScanner(conn)
	s.Buffer(make([]byte, 4096), 1<<20)
	for s.Scan() {
		var msg struct {
			ID    uint64          `json:"request_id"`
			Error string          `json:"error"`
			Data  json.RawMessage `json:"data"`
		}
		if json.Unmarshal(s.Bytes(), &msg) != nil || msg.ID == 0 {
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
