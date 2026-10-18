package mediainfo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestParseStreamsAndUnavailableValues(t *testing.T) {
	data := []byte(`{"format":{"format_name":"matroska,webm","duration":"12.5","bit_rate":"1500000"},"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1920,"height":1080,"avg_frame_rate":"30000/1001","bit_rate":"900000","tags":{"language":"eng","title":"Main"}},{"index":1,"codec_type":"audio","codec_name":"aac","sample_rate":"48000","channels":2,"channel_layout":"stereo","bit_rate":"128000"},{"index":2,"codec_type":"video","codec_name":"mjpeg","width":600,"height":600,"disposition":{"attached_pic":1}}]}`)
	p, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if p.Format != "matroska,webm" || p.Duration != "00:00:12.500" || len(p.Video) != 1 || len(p.Audio) != 1 || len(p.CoverArt) != 1 {
		t.Fatalf("parsed result: %#v", p)
	}
	if p.Video[0].Rate != "29.970 fps average" || p.Video[0].Width != 1920 || p.Video[0].Language != "eng" {
		t.Fatalf("video metadata: %#v", p.Video[0])
	}
	if !p.CoverArt[0].AttachedPicture || p.Audio[0].ChannelsText != "2 (stereo)" || p.Audio[0].Rate != "48000 Hz" {
		t.Fatalf("audio/cover metadata: %#v %#v", p.Audio[0], p.CoverArt[0])
	}
	missing, err := Parse([]byte(`{"format":{"duration":"N/A","bit_rate":"-1"},"streams":[{"codec_type":"video","avg_frame_rate":"1/0","width":0}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if missing.Duration != "Unknown" || missing.Bitrate != "Unknown" || missing.Video[0].Rate != "Unknown" || missing.Video[0].Codec != "Unknown" {
		t.Fatalf("invalid values: %#v", missing)
	}
}

func TestParsePreservesBitratePrecisionAndRoundsDuration(t *testing.T) {
	p, err := Parse([]byte(`{"format":{"duration":"3599.9996","bit_rate":"1500000"},"streams":[{"index":0,"codec_type":"audio","codec_name":"flac","bit_rate":"1411200"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Duration != "01:00:00.000" || p.Bitrate != "1.5 Mbit/s" || p.Audio[0].Bitrate != "1.41 Mbit/s" {
		t.Fatalf("normalized values: %#v", p)
	}
	arbitrary, err := Parse([]byte(`{"format":{"bit_rate":"2345678"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if arbitrary.Bitrate != "2.35 Mbit/s" {
		t.Fatalf("arbitrary bitrate=%q", arbitrary.Bitrate)
	}
	p, err = Parse([]byte(`{"format":{"duration":"59.9996"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Duration != "00:01:00.000" {
		t.Fatalf("rounded duration=%q", p.Duration)
	}
}

func TestInspectFilesystemAndProbeFallback(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.mp4")
	if err := os.WriteFile(target, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2024, 2, 3, 4, 5, 6, 0, time.FixedZone("test", 2*3600))
	if err := os.Chtimes(target, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linked.mp4")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	i := Inspector{LookPath: func(string) (string, error) { return "", os.ErrNotExist }}
	info, err := i.Inspect(context.Background(), link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Path != link || info.Name != "linked.mp4" || info.Size != 4 || !info.Modified.Equal(stamp) || !strings.Contains(info.Warning, "sudo apt install ffmpeg") {
		t.Fatalf("inspection: %#v", info)
	}
	empty := filepath.Join(dir, "empty.mp4")
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatal(err)
	}
	info, err = i.Inspect(context.Background(), empty)
	if err != nil || info.Size != 0 || !strings.Contains(info.Warning, "empty file") {
		t.Fatalf("empty file: %#v, %v", info, err)
	}
	if _, err = i.Inspect(context.Background(), filepath.Join(dir, "broken.mp4")); err == nil {
		t.Fatal("broken link target accepted")
	}
	broken := filepath.Join(dir, "broken-link.mp4")
	if err := os.Symlink(filepath.Join(dir, "missing"), broken); err != nil {
		t.Fatal(err)
	}
	if _, err = i.Inspect(context.Background(), broken); err == nil {
		t.Fatal("broken symlink accepted")
	}
}

func TestInspectProbeArgumentsAndMalformedResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "odd name.mp4")
	if err := os.WriteFile(path, []byte("media"), 0600); err != nil {
		t.Fatal(err)
	}
	var args []string
	i := Inspector{LookPath: func(string) (string, error) { return "/usr/bin/ffprobe", nil }, Run: func(_ context.Context, bin string, a ...string) ([]byte, []byte, error) {
		if bin != "/usr/bin/ffprobe" {
			t.Fatalf("binary=%q", bin)
		}
		args = append(args, a...)
		return []byte("{"), nil, nil
	}}
	info, err := i.Inspect(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(info.Warning, "invalid data") || args[len(args)-1] != path || strings.Contains(strings.Join(args, " "), "-show_frames") {
		t.Fatalf("warning=%q args=%v", info.Warning, args)
	}
	if !strings.Contains(strings.Join(args, " "), "-protocol_whitelist file,crypto,data") {
		t.Fatalf("protocol restriction missing: %v", args)
	}
}

func TestInspectCancellationSkipsQueuedFilesystemWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New().Inspect(ctx, "/does/not/matter")
	if err == nil || err != context.Canceled {
		t.Fatalf("cancelled inspection error=%v", err)
	}
}

func TestInspectReportsTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "slow.mp4")
	if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	i := Inspector{LookPath: func(string) (string, error) { return "fake", nil }, Timeout: 20 * time.Millisecond, Run: func(ctx context.Context, _ string, _ ...string) ([]byte, []byte, error) {
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}}
	start := time.Now()
	info, err := i.Inspect(context.Background(), path)
	if err != nil || !strings.Contains(info.Warning, "timed out") || time.Since(start) > time.Second {
		t.Fatalf("timeout result=%#v err=%v elapsed=%s", info, err, time.Since(start))
	}
}

func TestDefaultRunBoundsOutputAndReturnsExitStatus(t *testing.T) {
	stdoutScript := writeExecutable(t, "#!/bin/sh\ndd if=/dev/zero bs=1048577 count=1 2>/dev/null\n")
	if _, _, err := defaultRun(context.Background(), stdoutScript); err == nil || !strings.Contains(err.Error(), "stdout exceeded 1 MiB") {
		t.Fatalf("stdout limit error=%v", err)
	}
	stderrScript := writeExecutable(t, "#!/bin/sh\nhead -c 18432 /dev/zero >&2\n")
	_, diag, err := defaultRun(context.Background(), stderrScript)
	if err != nil || len(diag) != maxStderr {
		t.Fatalf("stderr bound: length=%d err=%v", len(diag), err)
	}
	failureScript := writeExecutable(t, "#!/bin/sh\nexit 7\n")
	if _, _, err = defaultRun(context.Background(), failureScript); err == nil {
		t.Fatal("nonzero child exit was reported as success")
	}
}

func TestDefaultRunCancellationKillsAndReapsChild(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	sleepScript := writeExecutable(t, "#!/bin/sh\nprintf '%s' \"$$\" > \"$1\"\nexec sleep 60\n")
	ctx, cancel := context.WithCancel(context.Background())
	start := time.Now()
	type result struct{ err error }
	done := make(chan result, 1)
	go func() { _, _, err := defaultRun(ctx, sleepScript, pidFile); done <- result{err} }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(pidFile); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			<-done
			t.Fatal("helper child did not start before deadline")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	var runErr error
	select {
	case got := <-done:
		runErr = got.err
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled process did not return promptly")
	}
	if runErr == nil || time.Since(start) > 6*time.Second {
		t.Fatalf("cancelled process did not return promptly: err=%v elapsed=%s", runErr, time.Since(start))
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("helper did not start: %v", err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("child %d is still present after defaultRun returned: %v", pid, err)
	}
}

func writeExecutable(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "helper.sh")
	if err := os.WriteFile(path, []byte(content), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}
