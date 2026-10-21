package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"mvplayer/internal/mediainfo"
)

func (m Model) View() tea.View {
	if m.quitting {
		v := tea.NewView("Goodbye.")
		v.AltScreen = true
		return v
	}
	var lines []string
	if m.width < 24 || m.height < 10 {
		lines = []string{fmt.Sprintf("MVPLAYER: terminal too small (%d x %d)", m.width, m.height), "Resize to at least 24 x 10 to continue.", "q quit  •  Ctrl+C quit"}
	} else if m.help {
		lines = m.helpView()
	} else if m.infoOpen {
		lines = m.infoView()
	} else {
		lines = m.mainView()
	}
	for len(lines) > m.height {
		lines = lines[:len(lines)-1]
	}
	for i := range lines {
		lines[i] = fit(lines[i], m.width)
	}
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	return v
}

func (m Model) mainView() []string {
	wide := m.width >= 100
	listRows := m.listRows()
	lines := []string{"MVPLAYER"}
	if wide {
		left := (m.width - 3) / 2
		right := m.width - 3 - left
		lines = append(lines, m.widePaneTitle("Browser", browserPane, m.cursor, len(m.visible), left)+" | "+m.widePaneTitle("Queue", queuePane, m.queueCursor, len(m.queue), right))
		browserContext := safeText(m.browseDir) + "  " + m.browserModeLabel()
		if m.recursive {
			browserContext += "  " + m.folderVisibilityLabel()
		}
		if m.loading {
			browserContext += "  Scanning..."
		}
		if m.filtering {
			browserContext += "  Filter (editing): " + safeText(m.query) + "_"
		} else if m.query != "" {
			browserContext += "  Filter: " + safeText(m.query)
		}
		queueContext := ""
		lines = append(lines, fit(browserContext, left)+" | "+fit(queueContext, right))
		browserRows := m.browserRows(m.browserOffset, listRows, left)
		queueRows := m.queueRows(m.queueOffset, listRows, right)
		for i := 0; i < listRows; i++ {
			l, r := "", ""
			if i < len(browserRows) {
				l = browserRows[i]
			}
			if i < len(queueRows) {
				r = queueRows[i]
			}
			lines = append(lines, fit(l, left)+" | "+fit(r, right))
		}
	} else {
		header := m.narrowPaneName("Browser", browserPane) + "  " + m.narrowPaneName("Queue", queuePane)
		if m.width >= 30 {
			header += "  Tab switch"
		}
		lines = append(lines, header)
		position, context := "0/0", safeText(m.browseDir)+"  "+m.browserModeLabel()
		if m.recursive {
			context += "  " + m.folderVisibilityLabel()
		}
		if m.loading {
			context += "  Scanning..."
		}
		if m.focus == browserPane {
			position = panePosition(m.cursor, len(m.visible))
			if m.filtering {
				context += "  Filter (editing): " + safeText(m.query) + "_"
			} else if m.query != "" {
				context += "  Filter: " + safeText(m.query)
			}
			lines = append(lines, position+"  "+context)
			lines = append(lines, m.browserRows(m.browserOffset, listRows, m.width)...)
		} else {
			lines = append(lines, context)
			position = panePosition(m.queueCursor, len(m.queue))
			lines = append(lines, position+"  Queue")
			lines = append(lines, m.queueRows(m.queueOffset, listRows, m.width)...)
		}
	}
	for len(lines) < m.height-5 {
		lines = append(lines, "")
	}
	lines = append(lines, m.statusLine(), m.mediaLine(), m.browserNoticeLine(), m.errorLine("Playback error", m.playbackErr), m.hints())
	return lines
}

func (m Model) browserRows(offset, count, width int) []string {
	var rows []string
	end := min(len(m.visible), offset+count)
	for i := offset; i < end; i++ {
		mark := "  "
		if i == m.cursor {
			mark = "> "
		}
		name := safeText(m.visible[i].Name)
		if m.visible[i].RelativePath != "" && !m.visible[i].Dir {
			name = safeText(m.visible[i].RelativePath)
		}
		if m.visible[i].Dir {
			name += "/"
		}
		row := mark + name
		if i == m.cursor && m.focus == browserPane {
			row = m.styleSelection("> " + name)
		}
		rows = append(rows, fit(row, width))
	}
	if len(rows) == 0 && m.loading {
		rows = append(rows, "Scanning...")
	} else if len(rows) == 0 {
		if m.recursive && !m.foldersVisible {
			rows = append(rows, "No matching media files. Press v to show folders.")
		} else {
			rows = append(rows, "No matching media files or directories.")
		}
	}
	return rows
}

func (m Model) queueRows(offset, count, width int) []string {
	var rows []string
	end := min(len(m.queue), offset+count)
	for i := offset; i < end; i++ {
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
		name := safeText(filepath.Base(m.queue[i]))
		row := mark + name
		if i == m.queueCursor && m.focus == queuePane {
			row = m.styleSelection(mark + name)
		}
		rows = append(rows, fit(row, width))
	}
	if len(rows) == 0 {
		rows = append(rows, "Queue is empty. Press Tab to return to browser.")
	}
	return rows
}

func paneTitle(name string, focused bool, cursor, count int) string {
	if focused {
		name = "[" + name + "]"
	}
	return fmt.Sprintf("%s  %s", name, panePosition(cursor, count))
}

func panePosition(cursor, count int) string {
	if count <= 0 {
		return "0/0"
	}
	return fmt.Sprintf("%d/%d", max(0, cursor)+1, count)
}

func (m Model) widePaneTitle(name string, which pane, cursor, count, width int) string {
	title := paneTitle(name, m.focus == which, cursor, count)
	if m.focus != which {
		return fit(title, width)
	}
	return m.styleHeading(fit(title, width))
}

func (m Model) narrowPaneName(name string, which pane) string {
	if m.focus != which {
		return name
	}
	return m.styleHeading("[" + name + "]")
}

func (m Model) styleHeading(s string) string {
	if m.colorProfile == colorprofile.Unknown || m.colorProfile == colorprofile.NoTTY {
		return s
	}
	if m.noColor || m.colorProfile == colorprofile.ASCII {
		return "\x1b[1m" + s + "\x1b[0m"
	}
	if m.colorProfile >= colorprofile.ANSI {
		return "\x1b[1;97;44m" + s + "\x1b[0m"
	}
	return s
}

func (m Model) styleSelection(s string) string {
	if m.colorProfile == colorprofile.Unknown || m.colorProfile == colorprofile.NoTTY {
		return s
	}
	if m.noColor || m.colorProfile == colorprofile.ASCII {
		return "\x1b[1m" + s + "\x1b[0m"
	}
	if m.colorProfile >= colorprofile.ANSI {
		return "\x1b[1;4m" + s + "\x1b[0m"
	}
	return s
}

func (m Model) statusLine() string {
	state := "Idle"
	if m.status.Failure != nil || m.playbackErr != nil {
		state = "Failed"
	} else if m.pending != "" {
		switch {
		case m.awaitingPlayback:
			state = "Loading"
		case m.status.Ended:
			state = playbackEndLabel(m.status.EndReason)
		case m.status.Paused:
			state = "Paused"
		case m.status.Path == "" && !m.status.HasTime:
			state = "Stopped"
		default:
			state = "Playing"
		}
	}
	s := "Status: " + state
	if m.status.HasTime {
		s += "  " + formatTime(m.status.Time)
	}
	if m.status.HasDuration {
		s += "/" + formatTime(m.status.Duration)
	}
	s += fmt.Sprintf("  Volume: %.0f%%", m.status.Volume)
	return s
}

func (m Model) mediaLine() string {
	if m.pending == "" {
		return "Media: —"
	}
	return "Media: " + safeText(m.pending)
}

func (m Model) errorLine(label string, err error) string {
	if err == nil {
		return ""
	}
	return label + ": " + safeText(err.Error())
}

func (m Model) hints() string {
	if m.width < 30 {
		if m.filtering {
			return "Tab switch • finish filter"
		}
		return "Tab switch • ? help"
	}
	if m.filtering {
		return "Ctrl+C quit • Filter • type to search • Enter/Esc finish • Ctrl+U clear • Tab switch"
	}
	if m.focus == browserPane {
		hint := "? help • i info • q quit • Tab • ↑↓ move • Enter open/play • a add • r mode • / filter"
		if m.recursive {
			hint += " • v folders"
		}
		return hint
	}
	return "Tab switch • ↑/k ↓/j move • Enter play • [/] select • d remove • n next • p previous • ? help • q quit"
}

func (m Model) infoView() []string {
	panelWidth := min(m.width, 84)
	left := (m.width - panelWidth) / 2
	wrapped := m.infoWrappedLines()
	panelHeight := m.infoPanelHeight(len(wrapped))
	page := m.infoPageSize()
	offset := min(m.infoOffset, max(0, len(wrapped)-page))
	end := min(len(wrapped), offset+page)
	top := max(0, (m.height-panelHeight)/2)
	background := m.mainView()
	for len(background) < m.height {
		background = append(background, "")
	}
	background = background[:m.height]
	panel := []string{}
	title := " File information "
	topBorder := "┌" + title + strings.Repeat("─", max(0, panelWidth-ansi.StringWidth(title)-2)) + "┐"
	panel = append(panel, fit(topBorder, panelWidth))
	for _, line := range wrapped[offset:end] {
		panel = append(panel, "│"+fit(line, panelWidth-2)+"│")
	}
	for len(panel) < panelHeight-1 {
		panel = append(panel, "│"+fit("", panelWidth-2)+"│")
	}
	hint := infoHint(panelWidth)
	bottom := "└ " + hint + " " + strings.Repeat("─", max(0, panelWidth-ansi.StringWidth(hint)-4)) + "┘"
	panel = append(panel, fit(bottom, panelWidth))
	for i, line := range panel {
		if y := top + i; y >= 0 && y < m.height {
			background[y] = overlayLine(background[y], line, left, panelWidth, m.width)
		}
	}
	return background
}

func (m Model) infoWrappedLines() []string {
	content := []string{"Name: " + safeText(filepath.Base(m.infoPath)), "Path: " + safeText(m.infoPath)}
	if m.infoLoading {
		content = append(content, "", "Loading file and media information…")
	} else if m.infoErr != nil {
		content = append(content, "", "File error: "+safeText(m.infoErr.Error()))
	} else {
		content = append(content, fmt.Sprintf("Size: %s (%d bytes)", humanSize(m.info.Size), m.info.Size), "Modified: "+m.info.Modified.Format("2006-01-02 15:04:05 -0700"), "", "Container: "+value(m.info.Format), "Duration: "+value(m.info.Duration), "Overall bitrate: "+value(m.info.Bitrate), "", "Video streams")
		videoStreams := make([]mediainfo.Stream, 0, len(m.info.Video))
		for _, s := range m.info.Video {
			if !s.AttachedPicture {
				videoStreams = append(videoStreams, s)
			}
		}
		if len(videoStreams) == 0 {
			content = append(content, "  None")
		}
		for _, s := range videoStreams {
			content = append(content, fmt.Sprintf("  Stream %d: %s, %s×%s, %s, %s", s.Index, s.Codec, positiveDim(s.Width), positiveDim(s.Height), s.Rate, s.Bitrate))
			content = appendTags(content, s)
		}
		content = append(content, "", "Audio streams")
		if len(m.info.Audio) == 0 {
			content = append(content, "  None")
		}
		for _, s := range m.info.Audio {
			content = append(content, fmt.Sprintf("  Stream %d: %s, %s, %s channels, %s", s.Index, s.Codec, s.Rate, s.ChannelsText, s.Bitrate))
			content = appendTags(content, s)
		}
		if len(m.info.CoverArt) > 0 {
			content = append(content, "", "Cover art")
			for _, s := range m.info.CoverArt {
				content = append(content, fmt.Sprintf("  Stream %d: %s, %s×%s", s.Index, s.Codec, positiveDim(s.Width), positiveDim(s.Height)))
				content = appendTags(content, s)
			}
		}
		if m.info.Warning != "" {
			content = append(content, "", "Inspection warning: "+safeText(m.info.Warning))
		}
	}
	var wrapped []string
	for _, line := range content {
		wrapped = append(wrapped, strings.Split(ansi.Wrap(safeText(line), max(1, min(m.width, 84)-4), ""), "\n")...)
	}
	return wrapped
}

func (m Model) infoPanelHeight(contentLines int) int {
	return min(m.height-2, max(8, min(contentLines+3, m.height-2)))
}

func (m Model) infoPageSize() int  { return max(1, m.infoPanelHeight(len(m.infoWrappedLines()))-2) }
func (m Model) infoMaxOffset() int { return max(0, len(m.infoWrappedLines())-m.infoPageSize()) }
func infoHint(width int) string {
	if width < 50 {
		return "Esc/i/Enter • ↑↓"
	}
	return "Esc/i/Enter close • ↑↓/j/k scroll • PgUp/PgDn page"
}

func overlayLine(background, panel string, left, panelWidth, width int) string {
	background = fit(background, width)
	if left <= 0 && panelWidth >= width {
		return panel
	}
	before := ansi.Cut(background, 0, left)
	after := ansi.Cut(background, left+panelWidth, width)
	return before + panel + after
}
func value(s string) string {
	if s == "" {
		return "Unknown"
	}
	return safeText(s)
}
func positiveDim(n int) string {
	if n <= 0 {
		return "Unknown"
	}
	return fmt.Sprint(n)
}
func humanSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	v := float64(n)
	u := []string{"KiB", "MiB", "GiB", "TiB"}
	i := 0
	for v >= 1024 && i < len(u)-1 {
		v /= 1024
		i++
	}
	if i > 0 {
		i--
	}
	return fmt.Sprintf("%.2f %s", v, u[i])
}
func appendTags(lines []string, s mediainfo.Stream) []string {
	if s.Language != "Unknown" {
		lines = append(lines, "    Language: "+safeText(s.Language))
	}
	if s.Title != "Unknown" {
		lines = append(lines, "    Title: "+safeText(s.Title))
	}
	return lines
}

func (m Model) helpView() []string {
	content := []string{"MVPLAYER help", "", "Navigation", "  Tab                 Switch browser and queue focus", "  Up/Down or j/k      Move in the focused pane", "  Enter               Open/play browser item; play selected queue item", "  i                   Inspect selected browser media; Esc/i/Enter closes", "  Modal               Up/Down or j/k scroll; PgUp/PgDn page; other keys are ignored", "  Backspace           Open parent directory (browser focus)", "  r                   Toggle recursive browsing (browser focus)", "  v                   Show/hide recursive folder rows (browser focus)", "  /                   Filter displayed names or relative paths", "  Filter: printable keys search, Backspace deletes, Ctrl+U clears, Enter/Escape finishes", "", "Queue", "  a                   Add highlighted browser media to queue", "  [ / ]               Select queue entry", "  d                   Remove selected pending entry (Queue focus only)", "  n / p               Next / previous queue action", "  > marks a remembered row; * marks the playing queue entry; >* means both", "  Focused pane names use brackets and white text on blue when color is enabled", "  The active row uses bold underline with your terminal's default text color", "  NO_COLOR keeps bold text cues and removes terminal colors", "", "Playback", "  Space               Pause or resume", "  Left/Right or h/l   Seek five seconds", "  + / = / -           Adjust volume", "  s                   Stop", "  f                   Toggle fullscreen for video", "  Enter on browser media plays immediately and preserves pending queue entries", "  Queue advances after natural end-of-file; errors and stop do not advance", "", "Other", "  ? / Escape          Close help", "  q / Ctrl+C          Quit", "  Audio-only playback has no video window or fullscreen control."}
	if m.err != nil {
		content = append(content, "", "Browser error: "+m.err.Error())
	}
	if m.warning != "" {
		content = append(content, "Partial results: "+m.warning)
	}
	if m.playbackErr != nil {
		content = append(content, "Playback error: "+m.playbackErr.Error())
	}
	if m.focus == browserPane && m.cursor >= 0 && m.cursor < len(m.visible) {
		content = append(content, "Browser selection: "+m.visible[m.cursor].Path)
	}
	if m.focus == queuePane && m.queueCursor >= 0 && m.queueCursor < len(m.queue) {
		content = append(content, "Queue selection: "+m.queue[m.queueCursor])
	}
	var wrapped []string
	for _, line := range content {
		wrapped = append(wrapped, strings.Split(ansi.Wrap(safeText(line), max(1, m.width), ""), "\n")...)
	}
	content = wrapped
	page := max(1, m.height-1)
	if m.helpOffset > max(0, len(content)-page) {
		m.helpOffset = max(0, len(content)-page)
	}
	end := min(len(content), m.helpOffset+page)
	return append([]string(nil), content[m.helpOffset:end]...)
}

func (m Model) browserModeLabel() string {
	if m.recursive {
		return "Recursive"
	}
	return "Directory"
}

func (m Model) folderVisibilityLabel() string {
	if m.foldersVisible {
		return "Folders shown"
	}
	return "Folders hidden"
}

func (m Model) browserNoticeLine() string {
	if m.err != nil {
		return m.errorLine("Browser error", m.err)
	}
	if m.warning == "" {
		return ""
	}
	return "Partial results: " + safeText(m.warning)
}

func formatTime(seconds float64) string { n := int(seconds); return fmt.Sprintf("%d:%02d", n/60, n%60) }

func safeText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

func fit(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(s) > width {
		s = ansi.Truncate(s, width, "...")
	}
	if n := width - ansi.StringWidth(s); n > 0 {
		s += strings.Repeat(" ", n)
	}
	return s
}
