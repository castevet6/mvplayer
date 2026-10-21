# MVPLAYER code navigation

This index is a map for finding the code behind a behavior. Start with
[Architecture](ARCHITECTURE.md) for the system-level design and boundaries.

## Repository map

| Location | Responsibility | Start here |
| --- | --- | --- |
| `cmd/mvplayer/main.go` | Executable entry point, flags, dependency wiring, TUI lifecycle | `run` |
| `internal/app/args.go` | Startup argument validation and path resolution | `ResolveArgs`, `Startup` |
| `internal/tui/model.go` | Interaction state, keys, state transitions, async effects | `Model`, `Init`, `Update`, `updateKey` |
| `internal/tui/view.go` | Main, help, and information rendering | `View`, `mainView`, `infoView` |
| `internal/browser/browser.go` | Directory and recursive scans, row model, filtering | `Entry`, `List`, `ListRecursive`, `Filter` |
| `internal/playback/mpv.go` | Playback interface and managed mpv JSON IPC process | `Player`, `State`, `MPV` |
| `internal/mediainfo/mediainfo.go` | Filesystem and ffprobe metadata inspection and parsing | `Inspector.Inspect`, `Parse` |

Tests are beside implementation files as `*_test.go`. The README documents
user-visible keys and runtime requirements; `doc/PROJECT-PLAN.md` records
intended product scope, and `doc/IMPLEMENTATION-PLAN.md` contains acceptance
history.

## Follow the main flows

### Launch

1. `cmd/mvplayer.run` handles help/version and obtains the working directory.
2. `internal/app.ResolveArgs` validates and resolves the optional path.
3. `tui.NewWithContext` creates the model and the app context.
4. Bubble Tea calls `Model.Init`; initial directory loading and optional startup
   playback run as commands.

### Browse, filter, and navigate

1. Key handling and browser mode transitions are in `internal/tui/model.go`,
   primarily `updateKey`, `startDirectoryScan`, and `applyFilter`.
2. The scan command selects `browser.List` or `browser.ListRecursive`.
3. A `directoryResult` returns to `Model.Update`, which checks its request,
   applies rows/filtering, and restores the previous selection where possible.
4. `internal/tui/view.go` renders `visible` rows through `browserRows`.

For filesystem rules, start with `browser.List` and `browser.ListRecursive`;
for row order or matching rules, continue to `sortEntries` and `Filter`.

### Play a file or advance the queue

1. Browser/queue key handling emits `FileSelectedMsg` or updates queue state in
   `internal/tui/model.go`.
2. `Model.load` calls the `playback.Player` interface in a command.
3. `playback.MPV.Load` starts or reuses mpv and submits a load request over IPC.
4. `MPV` publishes observed `State` values; `listenPlayer` feeds them back to
   the model, which updates status or advances the queue after natural EOF.
5. `internal/tui/view.go` renders playback status and current media details.

When debugging a playback command, follow `Player` first, then the relevant
`MPV` method and its IPC reader/response handling. This keeps TUI behavior
separate from process and protocol details.

### Inspect media

1. The browser key transition opens the info panel and schedules `inspectInfo`.
2. `mediainfo.Inspector.Inspect` gathers stat data, optionally runs ffprobe, and
   parses output with `Parse`.
3. The model accepts the matching `inspectResult`; the information panel in
   `view.go` formats and scrolls the resulting `mediainfo.Info`.

## Where to change common behavior

| Change | Likely code |
| --- | --- |
| Add or revise a key binding or state transition | `internal/tui/model.go` (`updateKey` and helpers); corresponding help text in `internal/tui/view.go` |
| Change screen layout, labels, or formatting | `internal/tui/view.go` |
| Change supported media extensions or hidden-file rules | `internal/browser/browser.go` (`mediaExtensions`, `List`, `ListRecursive`) |
| Change recursive ordering, symlink, or warning behavior | `internal/browser/browser.go` (`ListRecursive`, `sortEntries`) |
| Change startup path semantics | `internal/app/args.go` and `cmd/mvplayer/main.go` |
| Change queue selection or EOF advancement | `internal/tui/model.go` (queue fields, playback message cases, `nextQueue`, `advanceQueue`) |
| Change playback controls, mpv options, or IPC lifecycle | `internal/playback/mpv.go`; keep the UI-facing contract in `Player`/`State` |
| Change displayed media fields or ffprobe parsing | `internal/mediainfo/mediainfo.go` and the info rendering in `internal/tui/view.go` |
| Change build/version behavior | `Makefile`, `cmd/mvplayer/main.go`, and root `VERSION` |

## State ownership at a glance

| State | Owner | Lifetime |
| --- | --- | --- |
| Current root, visible rows, filter, pane focus, queue | `tui.Model` | Current application session |
| mpv process, IPC socket, observed playback snapshot | `playback.MPV` | Player instance, with process restart after disconnect |
| One inspection result | `tui.Model` plus `mediainfo.Info` | Current info view/request |
| Startup browse/play targets | `app.Startup` | Startup handoff into the TUI |

The app has no persistence layer: queue contents and browser preferences reset
when the process exits.
