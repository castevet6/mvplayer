// Package mediainfo inspects local media files without involving playback.
package mediainfo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxStdout = 1 << 20
const maxStderr = 16 << 10

var inspectionSlot = make(chan struct{}, 1)

type Stream struct {
	Index                                                             int
	Type, Codec, Rate, ChannelsText, Layout, Bitrate, Language, Title string
	Width, Height                                                     int
	AttachedPicture                                                   bool
}
type Info struct {
	Path, Name                string
	Size                      int64
	Modified                  time.Time
	Format, Duration, Bitrate string
	Video, Audio              []Stream
	Warning                   string
}

type Runner func(context.Context, string, ...string) ([]byte, []byte, error)
type Inspector struct {
	LookPath func(string) (string, error)
	Run      Runner
}

func New() Inspector { return Inspector{LookPath: exec.LookPath, Run: defaultRun} }

func (i Inspector) Inspect(ctx context.Context, path string) (Info, error) {
	select {
	case inspectionSlot <- struct{}{}:
		defer func() { <-inspectionSlot }()
	case <-ctx.Done():
		return Info{}, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return Info{}, err
	}
	if !filepath.IsAbs(path) {
		if p, e := filepath.Abs(path); e == nil {
			path = p
		}
	}
	st, err := os.Stat(path)
	if err != nil {
		return Info{}, fmt.Errorf("file: %w", err)
	}
	if !st.Mode().IsRegular() {
		return Info{}, errors.New("file is not a regular file")
	}
	info := Info{Path: path, Name: filepath.Base(path), Size: st.Size(), Modified: st.ModTime()}
	if st.Size() == 0 {
		info.Warning = "empty file; media could not be inspected"
		return info, nil
	}
	look := i.LookPath
	if look == nil {
		look = exec.LookPath
	}
	probe, err := look("ffprobe")
	if err != nil {
		info.Warning = "ffprobe is unavailable; install it with: sudo apt install ffmpeg"
		return info, nil
	}
	run := i.Run
	if run == nil {
		run = defaultRun
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, diag, err := run(ctx, probe, "-v", "error", "-protocol_whitelist", "file,crypto,data", "-show_entries", "format=format_name,duration,bit_rate:stream=index,codec_type,codec_name,width,height,avg_frame_rate,sample_rate,channels,channel_layout,bit_rate:stream_disposition=attached_pic:stream_tags=language,title", "-of", "json", path)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			info.Warning = "ffprobe timed out after 10 seconds"
			return info, nil
		}
		info.Warning = "ffprobe: " + cleanDiagnostic(diag, err)
		return info, nil
	}
	parsed, err := Parse(out)
	if err != nil {
		info.Warning = "ffprobe returned invalid data: " + err.Error()
		return info, nil
	}
	info.Format, info.Duration, info.Bitrate = parsed.Format, parsed.Duration, parsed.Bitrate
	info.Video, info.Audio = parsed.Video, parsed.Audio
	return info, nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.Len()+n > b.limit {
		b.exceeded = true
		p = p[:max(0, b.limit-b.Len())]
	}
	_, _ = b.Buffer.Write(p)
	if b.exceeded && b.limit == maxStdout {
		return n, errors.New("output limit exceeded")
	}
	return n, nil
}
func defaultRun(ctx context.Context, bin string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var out, diag limitedBuffer
	out.limit = maxStdout
	diag.limit = maxStderr
	cmd.Stdout = &out
	cmd.Stderr = &diag
	err := cmd.Run()
	if out.exceeded {
		return nil, diag.Bytes(), errors.New("stdout exceeded 1 MiB")
	}
	if diag.exceeded {
		diag.Truncate(maxStderr)
	}
	return out.Bytes(), diag.Bytes(), err
}
func cleanDiagnostic(diag []byte, err error) string {
	s := strings.TrimSpace(string(diag))
	if s == "" {
		s = err.Error()
	}
	if len(s) > 500 {
		s = s[:500] + "…"
	}
	return strings.Map(func(r rune) rune {
		if r < ' ' || r == 127 {
			return ' '
		}
		return r
	}, s)
}

type parsed struct {
	Format, Duration, Bitrate string
	Video, Audio              []Stream
}

func Parse(data []byte) (parsed, error) {
	var raw struct {
		Format struct {
			Name     string `json:"format_name"`
			Duration string `json:"duration"`
			Bitrate  string `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			Index       int               `json:"index"`
			Type        string            `json:"codec_type"`
			Codec       string            `json:"codec_name"`
			Width       int               `json:"width"`
			Height      int               `json:"height"`
			FPS         string            `json:"avg_frame_rate"`
			Sample      string            `json:"sample_rate"`
			Channels    int               `json:"channels"`
			Layout      string            `json:"channel_layout"`
			Bitrate     string            `json:"bit_rate"`
			Disposition map[string]int    `json:"disposition"`
			Tags        map[string]string `json:"tags"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return parsed{}, err
	}
	p := parsed{Format: unknown(raw.Format.Name), Duration: duration(raw.Format.Duration), Bitrate: bitrate(raw.Format.Bitrate)}
	for _, s := range raw.Streams {
		v := Stream{Index: s.Index, Type: s.Type, Codec: unknown(s.Codec), Bitrate: bitrate(s.Bitrate), Language: unknown(s.Tags["language"]), Title: unknown(s.Tags["title"]), AttachedPicture: s.Disposition["attached_pic"] == 1, Width: s.Width, Height: s.Height, Rate: rate(s.FPS), Layout: unknown(s.Layout)}
		if v.AttachedPicture {
			p.Video = append(p.Video, v)
			continue
		}
		if s.Type == "video" {
			p.Video = append(p.Video, v)
		} else if s.Type == "audio" {
			v.ChannelsText = channels(s.Channels, s.Layout)
			v.Rate = positive(s.Sample)
			p.Audio = append(p.Audio, v)
		}
	}
	sort.Slice(p.Video, func(i, j int) bool { return p.Video[i].Index < p.Video[j].Index })
	sort.Slice(p.Audio, func(i, j int) bool { return p.Audio[i].Index < p.Audio[j].Index })
	return p, nil
}
func unknown(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "n/a") {
		return "Unknown"
	}
	return s
}
func positive(s string) string {
	n, e := strconv.ParseFloat(s, 64)
	if e != nil || n <= 0 || math.IsInf(n, 0) || math.IsNaN(n) {
		return "Unknown"
	}
	return strconv.FormatFloat(n, 'f', -1, 64) + " Hz"
}
func bitrate(s string) string {
	n, e := strconv.ParseFloat(s, 64)
	if e != nil || n <= 0 || math.IsInf(n, 0) || math.IsNaN(n) {
		return "Unknown"
	}
	units := []string{"bit/s", "kbit/s", "Mbit/s", "Gbit/s"}
	i := 0
	for n >= 1000 && i < len(units)-1 {
		n /= 1000
		i++
	}
	return fmt.Sprintf("%.0f %s", n, units[i])
}
func duration(s string) string {
	n, e := strconv.ParseFloat(s, 64)
	if e != nil || n < 0 || math.IsNaN(n) || math.IsInf(n, 0) {
		return "Unknown"
	}
	h := int64(n) / 3600
	m := int64(n) % 3600 / 60
	sec := n - float64(h*3600+m*60)
	return fmt.Sprintf("%02d:%02d:%06.3f", h, m, sec)
}
func rate(s string) string {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return "Unknown"
	}
	a, e1 := strconv.ParseFloat(parts[0], 64)
	b, e2 := strconv.ParseFloat(parts[1], 64)
	if e1 != nil || e2 != nil || a <= 0 || b <= 0 {
		return "Unknown"
	}
	return fmt.Sprintf("%.3f fps average", a/b)
}
func channels(n int, layout string) string {
	if n <= 0 {
		return "Unknown"
	}
	if layout != "" && unknown(layout) != "Unknown" {
		return fmt.Sprintf("%d (%s)", n, layout)
	}
	return strconv.Itoa(n)
}
