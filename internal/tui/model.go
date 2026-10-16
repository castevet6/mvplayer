package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"mvplayer/internal/browser"
	"mvplayer/internal/playback"

	tea "charm.land/bubbletea/v2"
)

type directoryResult struct {
	request uint64
	path    string
	entries []browser.Entry
	err     error
}
type loadResult struct {
	path string
	err  error
}
type playerStateMsg playback.State
type controlResult struct{ err error }

// FileSelectedMsg is emitted when the user presses Enter on a media file.
type FileSelectedMsg struct{ Path string }

type listDirectory func(string) ([]browser.Entry, error)

// Model owns browser state. Directory reads run as Bubble Tea commands.
type Model struct {
	browseDir        string
	entries          []browser.Entry
	visible          []browser.Entry
	cursor           int
	query            string
	filtering        bool
	err              error
	selected         string
	pending          string
	request          uint64
	list             listDirectory
	player           playback.Player
	playing          string
	status           playback.State
	awaitingPlayback bool
	queue            []string
	queueCursor      int
	queueIndex       int
	queueResumeIndex int
	quitting         bool
}

func New(browseDir, playFile string, player playback.Player) Model {
	m := NewWithLister(browseDir, browser.List)
	m.pending = playFile
	m.awaitingPlayback = playFile != ""
	m.player = player
	return m
}

// NewWithLister allows filesystem errors and delayed results to be exercised
// without relying on host permissions or a real filesystem race.
func NewWithLister(browseDir string, list func(string) ([]browser.Entry, error)) Model {
	return Model{browseDir: browseDir, list: list, request: 1, queueCursor: -1, queueIndex: -1}
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
	player := m.player
	return func() tea.Msg {
		if player == nil {
			return loadResult{path: path, err: fmt.Errorf("playback is unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return loadResult{path: path, err: player.Load(ctx, path)}
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
		m.err, m.entries = msg.err, msg.entries
		m.applyFilter()
		return m, nil
	case FileSelectedMsg:
		m.err = nil
		m.selected = ""
		m.pending, m.playing = msg.Path, ""
		// The selected file starts a new playback attempt. Do not let the prior
		// file's EOF/error status leak into this attempt while mpv switches files.
		m.status.Ended, m.status.EndReason = false, ""
		m.status.Ready = false
		m.status.HasTime, m.status.HasDuration = false, false
		m.awaitingPlayback = true
		if m.queueIndex >= 0 {
			m.queueResumeIndex = m.queueIndex + 1
		}
		m.queueIndex = -1
		return m, m.load(msg.Path)
	case loadResult:
		if msg.path != m.pending {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			m.playing = ""
			m.awaitingPlayback = false
		} else {
			m.err = nil
			m.playing = msg.path
			m.status.Ended, m.status.EndReason = false, ""
		}
		return m, nil
	case playerStateMsg:
		wasEnded := m.status.Ended
		m.status = playback.State(msg)
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
			m.err = msg.err
		}
		return m, nil
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m Model) updateKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.String() == "q" || key.String() == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
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
		default:
			if key.Text != "" {
				m.query += key.Text
			}
		}
		m.applyFilter()
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
			if m.cursor >= 0 && m.cursor < len(m.visible) && m.visible[m.cursor].Media {
				m.queue = append(m.queue, m.visible[m.cursor].Path)
				if m.queueCursor < 0 {
					m.queueCursor = 0
				}
			}
		case "d":
			if m.queueCursor >= 0 && m.queueCursor < len(m.queue) {
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
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor+1 < len(m.visible) {
			m.cursor++
		}
	case "/":
		m.filtering = true
	case "backspace":
		parent := filepath.Dir(m.browseDir)
		if parent != m.browseDir {
			m.browseDir = parent
			m.request++
			m.entries, m.visible, m.cursor, m.err, m.query = nil, nil, 0, nil, ""
			return m, m.loadDirectory()
		}
	case "enter":
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
	m.err, m.pending, m.playing = nil, path, ""
	m.queueIndex = index
	m.queueResumeIndex = index + 1
	m.awaitingPlayback = true
	m.status.Ended, m.status.EndReason = false, ""
	m.status.Ready = false
	m.status.HasTime, m.status.HasDuration = false, false
	return m.load(path)
}

func (m *Model) applyFilter() {
	m.visible = browser.Filter(m.entries, m.query)
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m Model) View() tea.View {
	if m.quitting {
		view := tea.NewView("Goodbye.\n")
		view.AltScreen = true
		return view
	}
	var b strings.Builder
	fmt.Fprintf(&b, "MVPLAYER  %s\n\n", m.browseDir)
	if m.filtering {
		fmt.Fprintf(&b, "Filter: %s▏  (Enter/Esc to finish, Ctrl+U to clear)\n\n", m.query)
	} else if m.query != "" {
		fmt.Fprintf(&b, "Filter: %s  (press / to edit)\n\n", m.query)
	}
	if m.err != nil {
		fmt.Fprintf(&b, "Error: %v\n\n", m.err)
	}
	if len(m.visible) == 0 && m.err == nil {
		b.WriteString("No matching media files or directories.\n")
	}
	for i, entry := range m.visible {
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		name := entry.Name
		if entry.Dir {
			name += "/"
		}
		fmt.Fprintf(&b, "%s%s\n", marker, name)
	}
	if m.selected != "" {
		fmt.Fprintf(&b, "\nSelected: %s\n", m.selected)
	}
	if len(m.queue) > 0 {
		b.WriteString("\nQueue:\n")
		for i, path := range m.queue {
			mark := "  "
			if i == m.queueCursor {
				mark = "> "
			}
			if i == m.queueIndex {
				if i == m.queueCursor {
					mark = ">*"
				} else {
					mark = "* "
				}
			}
			fmt.Fprintf(&b, "%s%s\n", mark, filepath.Base(path))
		}
	}
	if m.pending != "" {
		stateLabel := "Playing"
		switch {
		case m.awaitingPlayback:
			stateLabel = "Loading"
		case m.status.Ended:
			stateLabel = playbackEndLabel(m.status.EndReason)
		case m.status.Paused:
			stateLabel = "Paused"
		case m.status.Path == "" && m.status.HasTime == false:
			stateLabel = "Stopped"
		}
		fmt.Fprintf(&b, "\nNow playing: %s  %s", m.pending, stateLabel)
		if m.status.HasTime {
			fmt.Fprintf(&b, "  %s", formatTime(m.status.Time))
		}
		if m.status.HasDuration {
			fmt.Fprintf(&b, "/%s", formatTime(m.status.Duration))
		}
		fmt.Fprintf(&b, "  Volume: %.0f%%\n", m.status.Volume)
	}
	b.WriteString("\n↑/k, ↓/j move • Enter open/play • a enqueue • [/] queue select • d remove • n next • p previous • Backspace parent • / filter • Space pause • ←/→ seek • +/- volume • s stop • f fullscreen • q quit\n")
	view := tea.NewView(b.String())
	view.AltScreen = true
	return view
}

func formatTime(seconds float64) string { n := int(seconds); return fmt.Sprintf("%d:%02d", n/60, n%60) }
func playbackEndLabel(reason string) string {
	switch reason {
	case "eof", "quit":
		return "Ended"
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
