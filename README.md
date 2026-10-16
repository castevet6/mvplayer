# MVPLAYER

MVPLAYER (MyVideoPlayer) is a keyboard-driven Linux video and audio player in
development. The planned player uses a Bubble Tea terminal interface for local
file browsing and queue management, with mpv handling media playback.

## Current status

Milestones M1 and M2 provide the CLI contract and responsive local file browser.
M3 connects CLI file paths and browser selections to one managed mpv process.
Video opens in its own window; audio plays without one, including files with
embedded cover art. See
[IMPLEMENTATION-PLAN.md](IMPLEMENTATION-PLAN.md) for milestone progress and
[PROJECT-PLAN.md](PROJECT-PLAN.md) for the intended behavior.

## Requirements

- Go 1.26 or newer
- mpv will be required starting with the playback milestone (M3)
- A terminal for the interactive interface

## Build and run

From the project root:

```sh
make test
make
```

`make` builds the `build/mvplayer` executable. Build outputs live in the ignored
`build/` directory. `make run` starts
the interactive player, and `make run ARGS="/path/to/media-file"` passes a
path argument. Environment variables such as `GOCACHE=/tmp/mvplayer-gocache`
can be set before `make` or `make run` and are inherited by Go.

Then run:

```sh
./build/mvplayer --help
./build/mvplayer
./build/mvplayer /path/to/directory
./build/mvplayer /path/to/media-file
```

Use Enter on a media file to start it. Press `q` or Ctrl+C to quit. Playback
requires mpv; browsing remains available when mpv is not installed.

The TUI shows the browser and queue side by side in terminals at least 100 columns
wide. Narrower terminals show the focused pane; press Tab to switch panes. Use
Up/Down or `j`/`k` to move in the focused pane. Press `?` for scrollable shortcut
help.

Press `a` on a browser media entry to append it to the in-memory queue. Use `[` and
`]` to select a queue entry and `d` to remove a pending entry. Enter on a queue
entry plays it; `n` starts the next pending item or advances, and `p` selects the
previous queue item (or resumes the next pending item when immediate playback is
active). Enter on a browser file plays immediately without changing pending queue
entries. The queue accepts duplicates and advances after natural EOF.

Press `/` in the browser to filter the current directory. Type any printable
characters, including shortcut letters, then press Enter or Escape to finish.
Backspace deletes while filtering and navigates to the parent directory otherwise;
Ctrl+U clears the filter.

During playback, these shortcuts work in both the terminal and the video window:
Space toggles pause, Left/Right (or `h`/`l`) seek by five seconds, `+`/`-`
adjust volume (`=` also raises it), `s` stops playback, and `f` toggles fullscreen
for video. The status line follows player-reported playback time, duration, volume,
and pause state. Playback controls work from either pane. A very small terminal
shows resize guidance while keeping keyboard controls active.

Closing the video window stops the current item and leaves the browser open.
If mpv exits or its IPC connection fails, MVPLAYER reports the failure and
keeps the queue; choose a file or queue item again to start a fresh player.
Quitting or interrupting MVPLAYER cancels pending playback work and closes its
managed mpv process and private socket.

Playback allows software graphics rendering when GPU acceleration is unavailable,
including remote desktops. This avoids a failing graphics fallback in mpv 0.40
that can prevent a video from reopening after playback ends or stops.

## Debian playback dependency

Install mpv on Debian with:

```sh
sudo apt install mpv
```

The initial release targets Debian desktop systems using either Wayland or X11.
