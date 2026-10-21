# MVPLAYER architecture

MVPLAYER is a local media browser and player with a keyboard-driven terminal
interface. The Go application owns navigation, filtering, queue state, metadata
presentation, and playback controls. It delegates media decoding and the video
window to an external `mpv` process. The UI is built with Bubble Tea v2.

## Runtime shape

```text
cmd/mvplayer
  ├─ internal/app       validates and resolves the launch path
  ├─ internal/tui       owns application state and renders the terminal UI
  │    ├─ internal/browser    lists and filters filesystem entries
  │    ├─ internal/playback   Player interface and asynchronous state events
  │    └─ internal/mediainfo  local stat data and optional ffprobe inspection
  └─ internal/playback       manages mpv process and private JSON IPC socket
```

`cmd/mvplayer` is the composition root. It handles the top-level help and
version flags, gets the working directory, resolves the optional path with
`app.ResolveArgs`, clears the terminal, constructs an `MPV`, and runs the
Bubble Tea program with a shared application context. On exit it cancels that
context and closes the player, joining UI and player shutdown errors.

`internal/app` is deliberately small: `Startup` carries the directory to browse
and an optional file to start. Relative paths are resolved from the supplied
working directory. A directory must be readable; a file must be regular and
openable. A file launch browses its parent and places that file into the initial
playback flow.

## UI state and effects

`internal/tui.Model` is the state owner. It tracks the browser root and rows,
selection and filter, directory/recursive mode, focus and scrolling for the
browser and queue, playback status, metadata panel state, and outstanding
operation identifiers. `Init`, `Update`, and `View` form the Bubble Tea loop:

- `Init` starts the initial directory load, subscribes to player events, and
  loads the startup file when present.
- `Update` handles keys and window/profile messages, applies asynchronous
  results, rejects stale directory, inspection, and playback results, and
  schedules follow-up commands.
- `View` renders the main panes, help, or media information view from the
  current model. `view.go` contains rendering and terminal-width-aware layout;
  interaction and state transitions live in `model.go`.

Filesystem reads, playback calls, and metadata inspection execute as Bubble Tea
commands so the update/render loop can continue responding to keys. Those
commands return typed messages to `Update`. Directory scans carry a monotonically
increasing request number, root path, and mode; only a result matching the
current request is applied. Playback attempts and metadata requests use similar
identifiers, preventing late results from replacing newer selections. Contexts
cancel scans, inspections, and superseded playback work when the relevant
operation changes or the application exits.

The queue is an in-memory list of file paths owned by the TUI model. It supports
duplicates and separates the queue cursor (selection) from the active queue
index. Direct browser playback does not mutate pending queue entries. Queue
advancement is driven by observed natural end-of-file state, while errors and
explicit stop remain distinct states.

## Filesystem browser

`internal/browser.Entry` is the shared row shape. `Path` identifies the target;
`Name` is the displayed/filtering name in ordinary directory mode;
`RelativePath` carries root-relative context for recursive results; `Dir` and
`Media` identify row kind.

`browser.List` reads one directory, omits hidden entries and unsupported files,
recognizes directories (including directory symlinks), and sorts rows
case-insensitively. `browser.ListRecursive` walks descendants with context
cancellation, excludes hidden paths, avoids descending through directory
symlinks, and returns immediate folders before media paths. A failure below the
root becomes a warning while readable subtrees remain available; failure at the
root is an error. `browser.Filter` performs case-insensitive substring matching
against the name or relative path.

The TUI owns the current browsing mode and folder visibility preference. It
calls the appropriate browser listing function, applies the query, and restores
selection by path after a refresh. Navigation into a folder changes the root;
the back action returns to its parent.

## Playback boundary and mpv lifecycle

`internal/playback.Player` is the interface consumed by the TUI: load, pause,
seek, volume, stop, fullscreen, event subscription, and close. `State` represents
the observed playback snapshot, including the attempt identifier, media path,
pause/end/readiness, failure, time, duration, volume, and fullscreen state.
Keeping this boundary narrow allows the TUI to depend on playback behavior
without depending on mpv's process or wire protocol.

`playback.MPV` starts mpv lazily when a load is requested. It creates a private,
permission-restricted temporary directory for the Unix-domain socket and input
bindings, starts the process without terminal output, connects to mpv's JSON IPC,
and observes the properties used by the status display. A reader loop decodes
events and resolves command replies; serialized writes and request IDs pair
responses with pending commands. The adapter manages process exit, disconnect,
restart, cleanup, and final close. The input bindings keep the video-window
shortcuts aligned with terminal playback controls.

The TUI calls `Player` methods in commands with bounded contexts, and consumes
the `Events` stream as Bubble Tea messages. Attempt IDs let it ignore events
from a previous file after a new selection. Playback is optional for browsing:
mpv absence becomes a playback error when playback is requested, while the
browser remains usable.

## Media information

`internal/mediainfo` inspects a local regular file independently from playback.
It always gathers filesystem metadata with `os.Stat`; when available, `ffprobe`
adds container, duration, bitrate, and audio/video stream details. The inspector
uses a bounded command timeout and output limits, and converts probe failures
into a warning in `Info` so file metadata can still be shown. A package-level
single-slot gate prevents concurrent probe processes. Parsing is isolated in
`Parse`, which normalizes unknown values and sorts streams by index.

The TUI opens the information view for a selected media file, cancels any
previous inspection, and applies only the latest request's result. Rendering,
wrapping, and scrolling for that view are in `internal/tui/view.go`.

## Dependencies and boundaries

The application uses Bubble Tea for terminal lifecycle and messages, plus
Charmbracelet terminal color/width support. Playback and probing are external
executables (`mpv` and optional `ffprobe`); media formats and codecs are supplied
by those installed tools. The Go code does not decode media itself. There is no
persistent database: queue and session preferences live only in the model.

Tests are colocated with each package. Browser and app tests exercise path and
listing behavior; TUI tests inject a lister/player and drive messages and keys;
playback tests exercise the mpv boundary and protocol behavior; mediainfo tests
cover parsing and probing through an injectable runner.

For package-by-package entry points and task-oriented paths through the code,
see [Code navigation](CODE-NAVIGATION.md).
