# MVPLAYER — Refined Project Design

## Purpose and First Milestone

MVPLAYER (MyVideoPlayer) is a keyboard-driven Linux video and audio player with a Bubble Tea terminal interface for browsing local files and managing a playback queue. Video appears in a separate mpv window, with fullscreen available. Audio-only files play without a separate window, using the TUI for controls and playback status.

The primary target is Debian desktop systems using either Wayland or X11. Implement application logic in Go, favoring the standard library. Use an installed mpv executable for media decoding, audio, and video output.

The first milestone includes local video and audio playback, browsing, mixed-media queue management, play/pause, seeking, volume, and fullscreen for video. Library indexing, streaming URLs, saved queues, resume positions, and dedicated subtitle/audio-track selection are deferred.

## User Experience

Support these launch forms:

- `mvplayer`: browse the current directory.
- `mvplayer <directory>`: browse the supplied directory.
- `mvplayer <file>`: start that video or audio file and browse its containing directory.
- `mvplayer --help`: display usage.

Show the file browser and queue side by side, with playback status and shortcut hints below. On narrow terminals, show the focused pane at full width and allow switching between browser and queue.

Support arrow keys and `j`/`k` navigation. Use Enter to open a directory or immediately play a file, a separate shortcut to enqueue, and Tab to switch panes. Provide directory navigation, current-directory filtering, queue removal, and next/previous controls.

Playing a file immediately interrupts the current media item while preserving pending queued items. A queue may contain both video and audio files, which play in insertion order. Playback stops when the queue ends; repeat and shuffle are deferred.

Playback shortcuts work in the terminal and, when present, the video window. Changes made through mpv must appear in the TUI. For audio-only playback, keep the video window closed, including for files with embedded cover art, and disable fullscreen controls. Transitioning between audio and video updates window visibility without interrupting queue advancement. Keyboard focus follows normal desktop behavior; global shortcuts are out of scope.

Closing the video window stops playback and leaves the browser open. Quitting MVPLAYER terminates its managed mpv process.

## Architecture and State

Separate responsibilities into three areas:

- **Browser:** directory listing, navigation, filtering, and file selection.
- **Application and TUI:** queue ownership, user actions, playback state, and rendering.
- **Playback adapter:** mpv process lifecycle, IPC commands, and playback notifications.

Use one managed mpv process, controlled through JSON IPC over a private Unix socket. Keep mpv protocol details behind a small Go interface exposing load, pause, seek, volume, fullscreen, stop, and shutdown operations, plus playback events.

MVPLAYER owns the queue; mpv reports actual playback status. Advance only after normal end-of-file, never after an explicit stop, window closure, or playback failure.

Run directory reads and IPC operations asynchronously. Deliver their results through Bubble Tea messages so application state changes remain serialized and the UI stays responsive.

Use the standard library for filesystem access, process management, sockets, and JSON. Add no database, background daemon, or alternative playback backend in this milestone.

## Errors and Validation

Show actionable errors inside the TUI for inaccessible directories, missing files, playback failures, and unexpected player exits. Preserve browsing and queue state. A failed queued item stops automatic advancement until the user retries or selects another item.

If mpv is unavailable, explain the dependency and how to install it. Bound startup and command waits, clean up sockets and processes on exit, and restore the terminal.

Validate:

- CLI startup with no argument, a directory, a file, and invalid input.
- Audio-only playback without a video window, including files with embedded cover art, and mixed queues transitioning in both directions between audio and video.
- Navigation, filtering, focus changes, and queue operations using Go tests.
- Playback events, command failures, and shutdown using a fake playback adapter.
- IPC response/event handling, including disconnects.
- Real playback on Debian under Wayland and X11, including fullscreen, focus changes, window closure, and clean exit.

## Defaults and Follow-up Work

Start with session-only state, name-sorted directory listings, hidden files omitted, and nonrecursive browsing by default. Optional recursive browsing is described in MVP-2. Include directories and recognized video and audio extensions, including common audio types such as `.mp3`, `.flac`, `.ogg`, `.opus`, `.m4a`, and `.wav`; mpv determines whether selected media is playable.

Keep initial styling and shortcuts built in. Defer user configuration, themes, packaging automation, and persistent history until the basic workflow is usable.

Implementation should begin with a working browser-to-mpv playback slice, followed by queue behavior and failure handling.
