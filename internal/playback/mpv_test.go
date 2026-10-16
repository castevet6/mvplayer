package playback

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMPVKeepsChildOutputOffTerminal(t *testing.T) {
	if os.Getenv("MVPLAYER_MPV_OUTPUT_HELPER") == "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		p := NewMPV()
		defer p.Close()
		// The fake mpv emits diagnostics and exits before creating IPC. Exercise
		// the real launch path without needing media, a display, or a socket.
		if err := p.Load(ctx, "/tmp/clip.mp4"); err == nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	dir := t.TempDir()
	const script = "#!/bin/sh\nprintf 'backend warning\\n' >&2\nprintf 'backend output\\n'\n"
	if err := os.WriteFile(filepath.Join(dir, "mpv"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestMPVKeepsChildOutputOffTerminal$")
	cmd.Env = append(os.Environ(), "MVPLAYER_MPV_OUTPUT_HELPER=1", "PATH="+dir+":"+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("playback helper failed: %v\n%s", err, output)
	}
	if len(output) != 0 {
		t.Fatalf("mpv wrote outside the TUI renderer: %q", output)
	}
}

func TestCommandCorrelatesResponseAndIgnoresEvents(t *testing.T) {
	client, peer := net.Pipe()
	peerDone := make(chan error, 1)
	go func() {
		defer peer.Close()
		s := bufio.NewScanner(peer)
		if !s.Scan() {
			peerDone <- s.Err()
			return
		}
		var request struct {
			ID uint64 `json:"request_id"`
		}
		if err := json.Unmarshal(s.Bytes(), &request); err != nil {
			peerDone <- err
			return
		}
		_, err := peer.Write([]byte(`{"event":"property-change","name":"pause","data":true}` + "\n" + `{"event":"end-file","reason":"eof"}` + "\n" + `{"event":"file-loaded"}` + "\n" + `{"request_id":` + jsonNumber(request.ID) + `,"error":"success","data":null}` + "\n"))
		peerDone <- err
	}()
	p := NewMPV()
	p.conn = client
	go p.readLoop(client)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := p.command(ctx, "loadfile", "/tmp/a.mp4", "replace"); err != nil {
		t.Fatal(err)
	}
	if err := <-peerDone; err != nil {
		t.Fatal(err)
	}
	select {
	case state := <-p.Events():
		if !state.Paused {
			t.Fatalf("property event not applied: %#v", state)
		}
	case <-time.After(time.Second):
		t.Fatal("property event not delivered")
	}
	select {
	case state := <-p.Events():
		if !state.Ended || state.EndReason != "eof" {
			t.Fatalf("end-file event not applied: %#v", state)
		}
	case <-time.After(time.Second):
		t.Fatal("end-file event not delivered")
	}
	select {
	case state := <-p.Events():
		if state.Ended || !state.Ready {
			t.Fatalf("file-loaded did not clear ended state and mark ready: %#v", state)
		}
	case <-time.After(time.Second):
		t.Fatal("file-loaded state not delivered")
	}
	_ = client.Close()
}

func TestCommandClassifiesBrokenPipeAsDisconnectedSession(t *testing.T) {
	client, peer := net.Pipe()
	_ = peer.Close()
	p := NewMPV()
	p.conn = client
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := p.command(ctx, "loadfile", "/tmp/repeat.mp4", "replace")
	if !errors.Is(err, errIPCDisconnected) {
		t.Fatalf("broken pipe should be recoverable as a disconnected session, got %v", err)
	}
	_ = client.Close()
}

func TestCommandClassifiesReadDisconnectAsDisconnectedSession(t *testing.T) {
	client, peer := net.Pipe()
	defer client.Close()
	p := NewMPV()
	p.conn = client
	go p.readLoop(client)
	go func() {
		defer peer.Close()
		// Let the write succeed, then disconnect while Load awaits its reply.
		s := bufio.NewScanner(peer)
		s.Scan()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := p.command(ctx, "loadfile", "/tmp/repeat.mp4", "replace")
	if !errors.Is(err, errIPCDisconnected) {
		t.Fatalf("read-side disconnect should recover on the first load, got %v", err)
	}
	<-p.closed
}

func TestUnavailablePathClearsCurrentFile(t *testing.T) {
	for _, raw := range []json.RawMessage{nil, json.RawMessage("null")} {
		p := NewMPV()
		p.state.Path = "/tmp/clip.mp4"
		p.updateProperty("path", raw)
		if state := <-p.Events(); state.Path != "" {
			t.Fatalf("unavailable path (%s) retained file: %+v", raw, state)
		}
	}
}

func TestMPVReloadsSameFileAfterEOFIntegration(t *testing.T) {
	if os.Getenv("MVPLAYER_MPV_INTEGRATION") != "1" {
		t.Skip("set MVPLAYER_MPV_INTEGRATION=1 to run the real mpv IPC check")
	}
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv is not installed")
	}
	path := os.Getenv("MVPLAYER_MPV_FIXTURE")
	if path == "" {
		path = filepath.Join(t.TempDir(), "short.wav")
		writeShortWAV(t, path)
	}
	p := NewMPV("--vo=null", "--ao=null")
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := p.Load(ctx, path); err != nil {
		t.Fatal(err)
	}
	// The generated clip is three seconds; leave enough time for natural EOF before
	// sending the same load request again.
	time.Sleep(3500 * time.Millisecond)
	for attempt := 0; attempt < 3; attempt++ {
		sawEnded := false
		for {
			select {
			case state := <-p.Events():
				if state.Ended && state.EndReason == "eof" {
					sawEnded = true
				}
			default:
				goto drained
			}
		}
	drained:
		if !sawEnded {
			t.Fatalf("playback attempt %d did not publish natural EOF", attempt)
		}
		if err := p.Load(ctx, path); err != nil {
			t.Fatalf("reload %d: %v", attempt+1, err)
		}
		deadline := time.NewTimer(2 * time.Second)
		started := false
	wait:
		for !started {
			select {
			case state := <-p.Events():
				started = state.Path == path && state.HasTime && state.Time > 0.02
			case <-deadline.C:
				break wait
			case <-ctx.Done():
				deadline.Stop()
				t.Fatal(ctx.Err())
			}
		}
		deadline.Stop()
		if !started {
			t.Fatalf("reload %d did not report advancing playback", attempt+1)
		}
		time.Sleep(3500 * time.Millisecond)
	}
}

func TestMPVWindowBindingsIntegration(t *testing.T) {
	if os.Getenv("MVPLAYER_MPV_INTEGRATION") != "1" {
		t.Skip("set MVPLAYER_MPV_INTEGRATION=1 to run the real mpv IPC check")
	}
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv is not installed")
	}
	path := os.Getenv("MVPLAYER_MPV_CONTROLS_FIXTURE")
	if path == "" {
		path = filepath.Join(t.TempDir(), "controls.wav")
		writeWAV(t, path, 20)
	}
	args := []string{"--vo=null", "--ao=null"}
	if os.Getenv("MVPLAYER_MPV_VIDEO_OUTPUT") == "1" {
		args = nil
	}
	logPath := filepath.Join(t.TempDir(), "mpv.log")
	args = append(args, "--log-file="+logPath)
	p := NewMPV(args...)
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := p.Load(ctx, path); err != nil {
		t.Fatal(err)
	}
	// IPC command success acknowledges dispatch. Wait for the actual effect,
	// especially for keypress, seek, and stop, which complete asynchronously.
	waitMPVState(t, p, func(s State) bool { return s.Path == path && s.HasTime && s.HasDuration })
	if err := p.Pause(ctx, true); err != nil {
		t.Fatal(err)
	}
	waitMPVState(t, p, func(s State) bool { return s.Paused })
	if err := p.SetVolume(ctx, 40); err != nil {
		t.Fatal(err)
	}
	waitMPVState(t, p, func(s State) bool { return s.Volume == 40 })
	for _, step := range []struct {
		key  string
		want func(State) bool
	}{
		{"+", func(s State) bool { return s.Volume == 45 }},
		{"=", func(s State) bool { return s.Volume == 50 }},
		{"-", func(s State) bool { return s.Volume == 45 }},
		{"RIGHT", func(s State) bool { return s.HasTime && s.Time >= 4.9 && s.Time < 6 }},
		{"LEFT", func(s State) bool { return s.HasTime && s.Time < 1 }},
		{"l", func(s State) bool { return s.HasTime && s.Time >= 4.9 && s.Time < 6 }},
		{"h", func(s State) bool { return s.HasTime && s.Time < 1 }},
		{"SPACE", func(s State) bool { return !s.Paused }},
		{"s", func(s State) bool { return s.Ended && s.EndReason == "stop" && s.Path == "" }},
	} {
		t.Run(step.key, func(t *testing.T) {
			if _, err := p.command(ctx, "keypress", step.key); err != nil {
				t.Fatal(err)
			}
			waitMPVState(t, p, step.want)
		})
		if t.Failed() {
			return
		}
	}
	// A window stop must not leave the next selection paused or disconnected.
	if err := p.Load(ctx, path); err != nil {
		t.Fatal(err)
	}
	waitMPVState(t, p, func(s State) bool { return s.Path == path && s.HasTime && !s.Ended && !s.Paused && s.Time > 0.1 })
}

func TestMPVRecoversReadDisconnectIntegration(t *testing.T) {
	if os.Getenv("MVPLAYER_MPV_INTEGRATION") != "1" {
		t.Skip("set MVPLAYER_MPV_INTEGRATION=1 to run the real mpv IPC check")
	}
	path := filepath.Join(t.TempDir(), "short.wav")
	writeShortWAV(t, path)
	client, peer := net.Pipe()
	p := NewMPV("--vo=null", "--ao=null")
	p.conn = client
	defer p.Close()
	go p.readLoop(client)
	go func() {
		defer peer.Close()
		s := bufio.NewScanner(peer)
		s.Scan()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := p.Load(ctx, path); err != nil {
		t.Fatalf("first load did not recover from read-side disconnect: %v", err)
	}
	waitMPVState(t, p, func(s State) bool { return s.Path == path && s.HasTime && s.Time > 0.1 })
}

func waitMPVState(t *testing.T, p *MPV, want func(State) bool) State {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	var last State
	for {
		select {
		case last = <-p.Events():
			if want(last) {
				return last
			}
		case <-deadline.C:
			t.Fatalf("timed out waiting for playback state; last state: %+v", last)
		}
	}
}

func writeShortWAV(t *testing.T, path string) {
	t.Helper()
	writeWAV(t, path, 3)
}

func writeWAV(t *testing.T, path string, seconds int) {
	t.Helper()
	const sampleRate = 8000
	samples := sampleRate * seconds
	dataSize := samples * 2
	b := make([]byte, 44+dataSize)
	copy(b[0:4], "RIFF")
	binary.LittleEndian.PutUint32(b[4:8], uint32(36+dataSize))
	copy(b[8:12], "WAVE")
	copy(b[12:16], "fmt ")
	binary.LittleEndian.PutUint32(b[16:20], 16)
	binary.LittleEndian.PutUint16(b[20:22], 1)
	binary.LittleEndian.PutUint16(b[22:24], 1)
	binary.LittleEndian.PutUint32(b[24:28], sampleRate)
	binary.LittleEndian.PutUint32(b[28:32], sampleRate*2)
	binary.LittleEndian.PutUint16(b[32:34], 2)
	binary.LittleEndian.PutUint16(b[34:36], 16)
	copy(b[36:40], "data")
	binary.LittleEndian.PutUint32(b[40:44], uint32(dataSize))
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}

func jsonNumber(n uint64) string { b, _ := json.Marshal(n); return string(b) }
