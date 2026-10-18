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

## Versioning

The current prerelease version is stored in the root `VERSION` file and follows
Semantic Versioning. Development builds report that version with the source
commit hash; a clean build from a commit tagged with the matching `v`-prefixed
version reports only the release version. For example, update `VERSION` to
`0.1.0-alpha.2`, commit the change, and tag that commit as `v0.1.0-alpha.2`.
Use `mvplayer --version` to inspect the version embedded in a build.

## Requirements

- Debian desktop with Wayland or X11
- Go 1.26 or newer to build
- mpv 0.40.0 was used for acceptance; other versions may behave differently
- `ffprobe` from Debian's `ffmpeg` package for file information (`i` in the Browser pane)
- A keyboard-driven terminal (tested with `TERM=xterm-256color`)
- Audio output for audible playback; a video-capable desktop session for video

Install the Debian playback dependency with:

```sh
sudo apt update
sudo apt install mpv
```

Install `ffprobe` to inspect media details with `i`:

```sh
sudo apt install ffmpeg
```

Install Go 1.26 or newer separately if the Go version provided by your Debian
release is older. The program does not bundle mpv or media codecs.

## Development and contribution tools

Building and testing the Go application requires Go 1.26 or newer. Git is used
for version information and branch work; GNU Make is used by the documented
`make` shortcuts. The project scripts use Bash.

The pull request helpers require the GitHub CLI (`gh`) and `jq`. Authenticate
`gh` with access to this repository before listing, creating, or updating pull
requests. Running the full Claude review with `scripts/review-pr.sh` also
requires the Claude Code CLI (`claude`), `curl`, OpenSSL (`openssl`), `flock`
(from util-linux), `realpath`, `awk`, and `grep`, along with standard GNU
utilities. Publishing a Claude review additionally requires the maintainer's
reviewer app configuration and key.

Run `make test-pr-workflow` to check the PR helper scripts and their mocked
GitHub API behavior. It requires Bash and `jq`.

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

To build and install MVPLAYER system-wide at `/usr/local/bin/mvplayer`, run
`./scripts/install.sh` or `make install` from the project directory. The script
builds the executable first, then uses `sudo` for the installation when needed.
You can remove the installed executable with `./scripts/uninstall.sh` or
`make uninstall`. Both scripts require Go 1.26 or newer to build; playback also
requires mpv.

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
wide. Narrower terminals show the focused pane, with both pane names in the header;
press Tab to switch panes. The focused pane name is bracketed, with white text on
a blue background when color is available. Its selected row uses bold underlined
text in the terminal's default foreground color. A plain `>` marks the remembered
selection in the other pane. The queue's `*` marks the playing item, so `>*`
means the playing item is also selected. Use Up/Down or
`j`/`k` to move in the focused pane. Press `?` for scrollable shortcut help.
Setting `NO_COLOR` disables colors while retaining the brackets and bold cues.

Press `a` on a browser media entry to append it to the in-memory queue. Use `[` and
`]` to select a queue entry and `d` to remove an eligible pending entry while
Queue is focused. Enter on a queue entry plays it; `n` starts the next pending
item or advances, and `p` selects the previous queue item (or resumes the next
pending item when immediate playback is active). Enter on a browser file plays
immediately without changing pending queue entries. The queue accepts duplicates
and advances after natural EOF.

Press `r` in the Browser pane to toggle recursive browsing. Directory mode lists
the current directory. Recursive mode shows immediate subfolders first, then
recognized media beneath the browsing root with root-relative paths. Enter on a
folder changes the root; Backspace returns to its parent while keeping the
selected mode. Recursive scans omit hidden paths and skip directory symlinks
under the current root. Immediate symlinked folders appear as rows and can be
opened explicitly.

Press `/` in the Browser pane to filter. The query matches names in Directory
mode and displayed relative paths in Recursive mode, including folder names.
Type printable characters, including shortcut letters, then press Enter or
Escape to finish. Backspace deletes while filtering and navigates to the parent
directory otherwise; Ctrl+U clears the filter.

Press `i` on a selected browser media file to inspect its file details, container,
duration, bitrate, and audio/video streams. The information view can be closed
with Escape, `i`, or Enter; use Up/Down or `j`/`k` to scroll and PageUp/PageDown
to move a page. Other keys are ignored while it is open, except `q` and Ctrl+C
to quit. Filesystem details remain available if `ffprobe` is missing or cannot
read the file; the view shows the installation command when needed.

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
omits hidden and unsupported entries, and does not provide streaming, subtitles
or track selection, repeat, shuffle, or saved queues. Global desktop shortcuts
are out of scope; video-window shortcuts work when that window has keyboard
focus.
