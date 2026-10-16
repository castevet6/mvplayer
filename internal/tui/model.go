package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"mvplayer/internal/browser"

	tea "charm.land/bubbletea/v2"
)

type directoryResult struct {
	request uint64
	path    string
	entries []browser.Entry
	err     error
}

// FileSelectedMsg is emitted when the user presses Enter on a media file.
type FileSelectedMsg struct{ Path string }

type listDirectory func(string) ([]browser.Entry, error)

// Model owns browser state. Directory reads run as Bubble Tea commands.
type Model struct {
	browseDir string
	entries   []browser.Entry
	visible   []browser.Entry
	cursor    int
	query     string
	filtering bool
	err       error
	selected  string
	pending   string
	request   uint64
	list      listDirectory
	quitting  bool
}

func New(browseDir, playFile string) Model {
	m := NewWithLister(browseDir, browser.List)
	m.pending = playFile
	return m
}

// NewWithLister allows filesystem errors and delayed results to be exercised
// without relying on host permissions or a real filesystem race.
func NewWithLister(browseDir string, list func(string) ([]browser.Entry, error)) Model {
	return Model{browseDir: browseDir, list: list, request: 1}
}

func (m Model) Init() tea.Cmd { return m.loadDirectory() }

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
		return tea.NewView("Goodbye.\n")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "MVPLAYER  %s\n\n", m.browseDir)
	if m.filtering {
		fmt.Fprintf(&b, "Filter: %s▏  (Enter/Esc to finish, Ctrl+U to clear)\n\n", m.query)
	} else if m.query != "" {
		fmt.Fprintf(&b, "Filter: %s  (press / to edit)\n\n", m.query)
	}
	if m.err != nil {
		fmt.Fprintf(&b, "Directory error: %v\n\n", m.err)
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
	if m.pending != "" {
		fmt.Fprintf(&b, "\nPending playback (available in M3): %s\n", m.pending)
	}
	b.WriteString("\n↑/k, ↓/j move • Enter open/play • Backspace parent • / filter • q quit\n")
	return tea.NewView(b.String())
}
