package playback

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
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
		_, err := peer.Write([]byte(`{"event":"file-loaded"}` + "\n" + `{"request_id":` + jsonNumber(request.ID) + `,"error":"success","data":null}` + "\n"))
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
	_ = client.Close()
}

func jsonNumber(n uint64) string { b, _ := json.Marshal(n); return string(b) }
