package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"mvplayer/internal/browser"
	"mvplayer/internal/playback"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

type directoryResult struct {
	request uint64
	path    string
	entries []browser.Entry
	err     error
}
type loadResult struct {
	path    string
	attempt uint64
	err     error
}
type playerStateMsg playback.State
type controlResult struct{ err error }

// FileSelectedMsg is emitted when the user presses Enter on a media file.
type FileSelectedMsg struct{ Path string }

type listDirectory func(string) ([]browser.Entry, error)

type pane uint8

const (
	browserPane pane = iota
	queuePane
)

// Model owns browser state. Directory reads run as Bubble Tea commands.
type Model struct {
	browseDir        string
	entries          []browser.Entry
	visible          []browser.Entry
	cursor           int
	query            string
	filtering        bool
	err              error
	playbackErr      error
	focus            pane
	width            int
	height           int
	browserOffset    int
	queueOffset      int
	help             bool
	helpOffset       int
	selected         string
	pending          string
	request          uint64
	list             listDirectory
	player           playback.Player
	playing          string
	status           playback.State
	awaitingPlayback bool
	attempt          uint64
	queue            []string
	queueCursor      int
	queueIndex       int
	queueResumeIndex int
	quitting         bool
	appContext       context.Context
	attemptContext   context.Context
	cancelAttempt    context.CancelFunc
	colorProfile     colorprofile.Profile
	noColor          bool
}

func New(browseDir, playFile string, player playback.Player) Model {
	return NewWithContext(context.Background(), browseDir, playFile, player)
}

func NewWithContext(ctx context.Context, browseDir, playFile string, player playback.Player) Model {
	m := NewWithLister(browseDir, browser.List)
	m.pending = playFile
	m.awaitingPlayback = playFile != ""
	if playFile != "" {
		m.attempt = 1
	}
	m.player = player
	m.appContext = ctx
	m.attemptContext, m.cancelAttempt = context.WithCancel(ctx)
	return m
}

// NewWithLister allows filesystem errors and delayed results to be exercised
// without relying on host permissions or a real filesystem race.
func NewWithLister(browseDir string, list func(string) ([]browser.Entry, error)) Model {
	noColorValue, noColorSet := os.LookupEnv("NO_COLOR")
	noColor := noColorSet && noColorValue != ""
	return Model{browseDir: browseDir, list: list, request: 1, queueCursor: -1, queueIndex: -1, width: 80, height: 24, noColor: noColor}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.loadDirectory()}
	if m.player != nil {
		cmds = append(cmds, m.listenPlayer())
	}
	if m.pending != "" {
		cmds = append(cmds, m.load(m.pending))
	}
	return tea.Batch(cmds...)
}

func (m Model) listenPlayer() tea.Cmd {
	events := m.player.Events()
	return func() tea.Msg {
		state, ok := <-events
		if !ok {
			return nil
		}
		return playerStateMsg(state)
	}
}

func (m Model) control(run func(context.Context) error) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return controlResult{run(ctx)}
	}
}

func (m Model) load(path string) tea.Cmd {
	player, attempt := m.player, m.attempt
	return func() tea.Msg {
		if player == nil {
			return loadResult{path: path, attempt: attempt, err: fmt.Errorf("playback is unavailable")}
		}
		base := m.attemptContext
		if base == nil {
			base = m.appContext
		}
		if base == nil {
			base = context.Background()
		}
		ctx, cancel := context.WithTimeout(base, 8*time.Second)
		defer cancel()
		return loadResult{path: path, attempt: attempt, err: player.Load(ctx, attempt, path)}
	}
}

func (m Model) loadDirectory() tea.Cmd {
	request, path, list := m.request, m.browseDir, m.list
	return func() tea.Msg {
		entries, err := list(path)
		return directoryResult{request: request, path: path, entries: entries, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case directoryResult:
		if msg.request != m.request || msg.path != m.browseDir {
			return m, nil
		}
		oldPath := ""
		if m.cursor >= 0 && m.cursor < len(m.visible) {
			oldPath = m.visible[m.cursor].Path
		}
		m.err, m.entries = msg.err, msg.entries
		m.applyFilter()
		m.restoreBrowserSelection(oldPath)
		return m, nil
	case tea.WindowSizeMsg:
		if msg.Width > 0 && msg.Height > 0 {
			m.width, m.height = msg.Width, msg.Height
			m.keepVisible()
		}
		return m, nil
	case tea.ColorProfileMsg:
		m.colorProfile = msg.Profile
		return m, nil
	case FileSelectedMsg:
		if m.cancelAttempt != nil {
			m.cancelAttempt()
		}
		base := m.appContext
		if base == nil {
			base = context.Background()
		}
		m.attemptContext, m.cancelAttempt = context.WithCancel(base)
		m.err, m.playbackErr = nil, nil
		m.selected = ""
		m.pending, m.playing = msg.Path, ""
		// The selected file starts a new playback attempt. Do not let the prior
		// file's EOF/error status leak into this attempt while mpv switches files.
		m.status.Ended, m.status.EndReason = false, ""
		m.status.Ready = false
		m.status.HasTime, m.status.HasDuration = false, false
		m.awaitingPlayback = true
		m.attempt++
		if m.queueIndex >= 0 {
			m.queueResumeIndex = m.queueIndex + 1
		}
		m.queueIndex = -1
		return m, m.load(msg.Path)
	case loadResult:
		if msg.path != m.pending || msg.attempt != m.attempt {
			return m, nil
		}
		if msg.err != nil {
			m.playbackErr = msg.err
			m.playing = ""
			m.awaitingPlayback = false
		} else {
			m.playbackErr = nil
			m.playing = msg.path
			m.status.Ended, m.status.EndReason = false, ""
		}
		return m, nil
	case playerStateMsg:
		if playback.State(msg).Attempt != 0 && playback.State(msg).Attempt != m.attempt {
			return m, m.listenPlayer()
		}
		wasEnded := m.status.Ended
		m.status = playback.State(msg)
		if m.status.Failure != nil {
			m.playbackErr = m.status.Failure
			m.awaitingPlayback = false
			m.playing = ""
			return m, m.listenPlayer()
		}
		if m.status.Path != "" {
			m.playing = m.status.Path
			m.pending = m.playing
			if m.awaitingPlayback && m.status.Path == m.pending && (m.status.Ready || m.status.HasTime) {
				m.awaitingPlayback = false
			}
		} else if m.status.Ended {
			// mpv clears its path just after emitting end-file. Preserve the
			// completed item so the status row can display Ended.
			m.playing = m.pending
		} else if !m.awaitingPlayback {
			m.playing, m.pending = "", ""
		}
		if !wasEnded && !m.awaitingPlayback && m.status.Ended && m.status.EndReason == "eof" && m.queueIndex >= 0 {
			return m, tea.Batch(m.advanceQueue(m.queueIndex+1), m.listenPlayer())
		}
		return m, m.listenPlayer()
	case controlResult:
		if msg.err != nil {
			m.playbackErr = msg.err
		}
		return m, nil
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m Model) updateKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.String() == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}
	if m.help {
		switch key.String() {
		case "?", "esc":
			m.help = false
		case "up", "k":
			if m.helpOffset > 0 {
				m.helpOffset--
			}
		case "down", "j":
			m.helpOffset++
		case "pgup":
			m.helpOffset = max(0, m.helpOffset-m.helpPageSize())
		case "pgdown":
			m.helpOffset += m.helpPageSize()
		case "q":
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}
	if m.filtering {
		switch key.String() {
		case "esc", "enter":
			m.filtering = false
		case "backspace":
			if len(m.query) > 0 {
				_, size := utf8.DecodeLastRuneInString(m.query)
				m.query = m.query[:len(m.query)-size]
			}
		case "ctrl+u":
			m.query = ""
		case "tab":
			m.filtering = false
			m.focus = queuePane
		default:
			if key.Text != "" {
				m.query += key.Text
			}
		}
		m.applyFilter()
		m.keepVisible()
		return m, nil
	}
	if key.String() == "q" {
		m.quitting = true
		return m, tea.Quit
	}
	if key.String() == "tab" {
		m.focus = 1 - m.focus
		m.keepVisible()
		return m, nil
	}
	if key.String() == "?" {
		m.help, m.helpOffset = true, 0
		return m, nil
	}
	if m.playing != "" && m.player != nil && !m.status.Ended {
		switch key.String() {
		case "space":
			return m, m.control(func(ctx context.Context) error { return m.player.Pause(ctx, !m.status.Paused) })
		case "left", "h":
			return m, m.control(func(ctx context.Context) error { return m.player.Seek(ctx, -5) })
		case "right", "l":
			return m, m.control(func(ctx context.Context) error { return m.player.Seek(ctx, 5) })
		case "+", "=":
			return m, m.control(func(ctx context.Context) error { return m.player.SetVolume(ctx, min(100, m.status.Volume+5)) })
		case "-":
			return m, m.control(func(ctx context.Context) error { return m.player.SetVolume(ctx, max(0, m.status.Volume-5)) })
		case "s":
			return m, m.control(func(ctx context.Context) error { return m.player.Stop(ctx) })
		case "f":
			if isVideo(m.playing) {
				return m, m.control(func(ctx context.Context) error { return m.player.Fullscreen(ctx, !m.status.Fullscreen) })
			}
		}
	}
	if m.awaitingPlayback && m.player != nil && key.String() == "s" {
		cancelAttempt := m.cancelAttempt
		return m, m.control(func(ctx context.Context) error {
			err := m.player.Stop(ctx)
			if cancelAttempt != nil {
				cancelAttempt()
			}
			return err
		})
	}
	if m.player != nil {
		switch key.String() {
		case "n":
			return m, m.nextQueue()
		case "p":
			if m.queueIndex > 0 {
				return m, m.advanceQueue(m.queueIndex - 1)
			}
			if m.queueIndex < 0 && m.queueResumeIndex < len(m.queue) {
				return m, m.advanceQueue(m.queueResumeIndex)
			}
		case "a":
			if m.focus == browserPane && m.cursor >= 0 && m.cursor < len(m.visible) && m.visible[m.cursor].Media {
				m.queue = append(m.queue, m.visible[m.cursor].Path)
				if m.queueCursor < 0 {
					m.queueCursor = 0
				}
			}
		case "d":
			if m.focus == queuePane && m.queueCursor >= 0 && m.queueCursor < len(m.queue) {
				if (m.queueIndex >= 0 && m.queueCursor <= m.queueIndex) || (m.queueIndex < 0 && m.queueCursor < m.queueResumeIndex) {
					break
				}
				m.queue = append(m.queue[:m.queueCursor], m.queue[m.queueCursor+1:]...)
				if m.queueIndex > m.queueCursor {
					m.queueIndex--
				}
				if m.queueResumeIndex > m.queueCursor {
					m.queueResumeIndex--
				}
				if m.queueCursor >= len(m.queue) {
					m.queueCursor = len(m.queue) - 1
				}
			}
		case "]":
			if m.queueCursor+1 < len(m.queue) {
				m.queueCursor++
			}
		case "[":
			if m.queueCursor > 0 {
				m.queueCursor--
			}
		}
	}
	switch key.String() {
	case "up", "k":
		if m.focus == browserPane {
			if m.cursor > 0 {
				m.cursor--
			}
		} else if m.queueCursor > 0 {
			m.queueCursor--
		}
	case "down", "j":
		if m.focus == browserPane {
			if m.cursor+1 < len(m.visible) {
				m.cursor++
			}
		} else if m.queueCursor+1 < len(m.queue) {
			m.queueCursor++
		}
	case "/":
		if m.focus == browserPane {
			m.filtering = true
		}
	case "backspace":
		if m.focus != browserPane {
			break
		}
		parent := filepath.Dir(m.browseDir)
		if parent != m.browseDir {
			m.browseDir = parent
			m.request++
			m.entries, m.visible, m.cursor, m.err, m.query = nil, nil, 0, nil, ""
			return m, m.loadDirectory()
		}
	case "enter":
		if m.focus == queuePane {
			if m.player != nil && m.queueCursor >= 0 && m.queueCursor < len(m.queue) {
				return m, m.advanceQueue(m.queueCursor)
			}
			break
		}
		if m.cursor >= 0 && m.cursor < len(m.visible) {
			entry := m.visible[m.cursor]
			if entry.Dir {
				m.browseDir = entry.Path
				m.request++
				m.entries, m.visible, m.cursor, m.err, m.query = nil, nil, 0, nil, ""
				return m, m.loadDirectory()
			}
			m.selected = entry.Path
			return m, func() tea.Msg { return FileSelectedMsg{Path: entry.Path} }
		}
	}
	m.keepVisible()
	return m, nil
}

func (m *Model) nextQueue() tea.Cmd {
	if m.queueIndex < 0 {
		return m.advanceQueue(m.queueResumeIndex)
	}
	return m.advanceQueue(m.queueIndex + 1)
}

func (m *Model) advanceQueue(index int) tea.Cmd {
	if index < 0 || index >= len(m.queue) {
		return nil
	}
	path := m.queue[index]
	if m.cancelAttempt != nil {
		m.cancelAttempt()
	}
	base := m.appContext
	if base == nil {
		base = context.Background()
	}
	m.attemptContext, m.cancelAttempt = context.WithCancel(base)
	m.err, m.playbackErr, m.pending, m.playing = nil, nil, path, ""
	m.queueIndex = index
	m.queueResumeIndex = index + 1
	m.awaitingPlayback = true
	m.attempt++
	m.status.Ended, m.status.EndReason = false, ""
	m.status.Ready = false
	m.status.HasTime, m.status.HasDuration = false, false
	return m.load(path)
}

func (m *Model) applyFilter() {
	selected := ""
	if m.cursor >= 0 && m.cursor < len(m.visible) {
		selected = m.visible[m.cursor].Path
	}
	m.visible = browser.Filter(m.entries, m.query)
	if selected != "" {
		for i := range m.visible {
			if m.visible[i].Path == selected {
				m.cursor = i
				break
			}
		}
	}
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) restoreBrowserSelection(path string) {
	if path == "" {
		return
	}
	for i := range m.visible {
		if m.visible[i].Path == path {
			m.cursor = i
			return
		}
	}
}

func (m *Model) keepVisible() {
	rows := max(1, m.height-8)
	m.browserOffset = visibleOffset(m.browserOffset, m.cursor, len(m.visible), rows)
	m.queueOffset = visibleOffset(m.queueOffset, m.queueCursor, len(m.queue), rows)
}

func visibleOffset(offset, cursor, count, rows int) int {
	if cursor < 0 || count == 0 {
		return 0
	}
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+rows {
		offset = cursor - rows + 1
	}
	return max(0, min(offset, max(0, count-rows)))
}

func (m Model) helpPageSize() int { return max(1, m.height-3) }
func playbackEndLabel(reason string) string {
	switch reason {
	case "eof":
		return "Ended"
	case "quit", "stop":
		return "Stopped"
	case "error":
		return "Failed"
	default:
		return "Stopped"
	}
}

func isVideo(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".mkv", ".mov", ".webm", ".avi", ".m4v", ".mpeg", ".mpg":
		return true
	}
	return false
}
