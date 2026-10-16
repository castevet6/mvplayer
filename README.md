# MVPLAYER

MVPLAYER (MyVideoPlayer) is a keyboard-driven Linux video and audio player in
development. The planned player uses a Bubble Tea terminal interface for local
file browsing and queue management, with mpv handling media playback.

## Current status

Milestone M1 provides the Go application foundation and CLI contract. It accepts
no argument, a directory, or a file path; a file is shown as pending playback.
The current screen is a minimal startup view that exits with `q` or Ctrl+C.
File browsing and media playback are not implemented yet. See
[IMPLEMENTATION-PLAN.md](IMPLEMENTATION-PLAN.md) for milestone progress and
[PROJECT-PLAN.md](PROJECT-PLAN.md) for the intended behavior.

## Requirements

- Go 1.26 or newer
- mpv will be required starting with the playback milestone (M3)
- A terminal for the interactive interface

## Build and run

From the project root:

```sh
go test ./...
go vet ./...
go build -o mvplayer ./cmd/mvplayer
```

Then run:

```sh
./mvplayer --help
./mvplayer
./mvplayer /path/to/directory
./mvplayer /path/to/media-file
```

The file form currently records a pending playback request; it does not start
mpv. Press `q` or Ctrl+C to exit the current interface.

## Debian playback dependency

mpv is not needed for the current M1 foundation. When playback arrives in M3,
install mpv on Debian with:

```sh
sudo apt install mpv
```

The initial release targets Debian desktop systems using either Wayland or X11.
