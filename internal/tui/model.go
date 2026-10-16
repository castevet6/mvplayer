package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Model is the minimal M1 terminal screen. Browser and playback behavior are
// added in later milestones.
type Model struct {
	browseDir string
	playFile  string
	quitting  bool
}

func New(browseDir, playFile string) Model {
	return Model{browseDir: browseDir, playFile: playFile}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	if m.quitting {
		return tea.NewView("Goodbye.\n")
	}
	var b strings.Builder
	b.WriteString("MVPLAYER\n\n")
	b.WriteString("Browsing: ")
	b.WriteString(m.browseDir)
	b.WriteByte('\n')
	if m.playFile != "" {
		b.WriteString("Pending playback (available in M3): ")
		b.WriteString(m.playFile)
		b.WriteByte('\n')
	}
	b.WriteString("\nM1 foundation only: media playback and file browsing are not implemented yet.\n")
	b.WriteString("Press q or Ctrl+C to quit.\n")
	return tea.NewView(b.String())
}
