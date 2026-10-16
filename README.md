# MVPLAYER

MVPLAYER (MyVideoPlayer) is a keyboard-driven Linux video and audio player in
development. The planned player uses a Bubble Tea terminal interface for local
file browsing and queue management, with mpv handling media playback.

## Current status

The initial workflow is implemented: browse and filter local files, build an
in-memory mixed-media queue, and control mpv playback from the terminal or video
window. Separate Debian Wayland and X11 session checks were not performed under
WSLg, which provides integrated GUI paths rather than two desktop sessions;
some physical desktop checks remain pending. See
[implementation plan](doc/IMPLEMENTATION-PLAN.md) for the acceptance record and
[project plan](doc/PROJECT-PLAN.md) for intended behavior.

## Requirements

- Debian desktop with Wayland or X11
- Go 1.26 or newer to build
- mpv 0.40.0 was used for acceptance; other versions may behave differently
- A keyboard-driven terminal (tested with `TERM=xterm-256color`)
- Audio output for audible playback; a video-capable desktop session for video

Install the Debian playback dependency with:

```sh
sudo apt update
sudo apt install mpv
```

Install Go 1.26 or newer separately if the Go version provided by your Debian
release is older. The program does not bundle mpv or media codecs.

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

Supported launch forms are the current directory, a directory, or a media file
(which starts playback and browses its containing directory). `--help` prints
usage without starting the TUI. Invalid or inaccessible paths are reported at
startup. Playback requires mpv; browsing remains available when mpv is not
installed. Use Enter on a browser media file to start it. Press `q` or Ctrl+C to
quit.

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
including remote desktops. Supported media formats depend on the codecs in the
installed mpv build. The queue and preferences are in-memory only. The player
browses one directory at a time, omits hidden and unsupported entries, and does
not provide streaming, subtitles or track selection, repeat, shuffle, or saved
queues. Global desktop shortcuts are out of scope; video-window shortcuts work
when that window has keyboard focus.
