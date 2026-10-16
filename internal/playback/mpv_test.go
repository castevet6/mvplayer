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
	"testing"
	"time"
)

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
		if state.Ended {
			t.Fatalf("file-loaded did not clear ended state: %#v", state)
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

func writeShortWAV(t *testing.T, path string) {
	t.Helper()
	const sampleRate, samples = 8000, 24000
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
