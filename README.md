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

`make` builds the `mvplayer` executable in the project root. `make run` starts
the interactive player, and `make run ARGS="/path/to/media-file"` passes a
path argument. Environment variables such as `GOCACHE=/tmp/mvplayer-gocache`
can be set before `make` or `make run` and are inherited by Go.

Then run:

```sh
./mvplayer --help
./mvplayer
./mvplayer /path/to/directory
./mvplayer /path/to/media-file
```

Use Enter on a media file to start it. Press `q` or Ctrl+C to quit. Playback
requires mpv; browsing remains available when mpv is not installed.

During playback, Space toggles pause, Left/Right seek by five seconds, `+`/`-`
adjust volume, `s` stops playback, and `f` toggles fullscreen for video. The
status line follows player-reported playback time, duration, volume, and pause
state. Browser movement remains on Up/Down or `j`/`k`.

## Debian playback dependency

Install mpv on Debian with:

```sh
sudo apt install mpv
```

The initial release targets Debian desktop systems using either Wayland or X11.
