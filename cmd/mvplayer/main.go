package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"mvplayer/internal/app"
	"mvplayer/internal/playback"
	"mvplayer/internal/tui"
)

const usage = `Usage: mvplayer [path]

Browse the current directory, a supplied directory, or start with a file
selected for playback. Playback requires mpv (sudo apt install mpv).

Options:
  -h, --help  show this help
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "mvplayer:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		_, err := fmt.Fprint(os.Stdout, usage)
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}
	startup, err := app.ResolveArgs(args, cwd)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(os.Stdout, "\x1b[2J\x1b[H"); err != nil {
		return fmt.Errorf("clear terminal before starting TUI: %w", err)
	}
	player := playback.NewMPV()
	_, err = tea.NewProgram(tui.New(startup.BrowseDir, startup.PlayFile, player)).Run()
	if closeErr := player.Close(); err == nil {
		err = closeErr
	}
	return err
}
