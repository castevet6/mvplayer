# MVPLAYER — Phased Implementation Plan

## Purpose and Current State

Follow this document when implementing the design in [PROJECT-PLAN.md](PROJECT-PLAN.md). That document defines product behavior; this document defines delivery order, verification gates, and progress. Its broad first milestone is split into the eight milestones below. All eight together deliver the initial release scope.

Current state: M1 is complete. The application has a tested Go foundation and CLI contract; directory browsing and real media playback are planned for later milestones. Audio and video are both part of the initial release.

## Working Through One Milestone

Work on one milestone at a time, in order. Each milestone follows these phases:

1. **Prepare:** Read the relevant design and existing code. Set the milestone to `in progress`, record its scope, and resolve any implementation choices needed for its acceptance cases. Record decisions here; update the project design only if product behavior changes.
2. **Implement:** Build the smallest complete behavior described by the milestone, keeping filesystem and player operations outside the TUI update loop. Add focused tests for observable behavior and failure cases.
3. **Verify:** Run the milestone's targeted tests, then the shared checks below. Perform its manual checks where the environment permits. Record commands, results, and any missing coverage.
4. **Close:** Compare results against every completion criterion. Fix failures within the milestone, update its status and evidence, and record remaining work and the next milestone. Stop at the milestone boundary unless further implementation is authorized.

Use statuses `not started`, `in progress`, `verification pending`, and `complete`. A required check that could not run remains pending; passing unit tests alone does not establish real playback or desktop compatibility. Independent work may continue after a pending environment check, but the affected milestone remains incomplete.

After each implementation session, update the progress table and append a work-log entry. Do not mark planned work as completed or overwrite earlier verification evidence.

## Progress Tracker

| Milestone | Deliverable | Status | Verification evidence |
| --- | --- | --- | --- |
| M1 | Go foundation and CLI contract | complete | `go test ./...` and `go vet ./...` passed; `go build -buildvcs=false ./...` passed; built executable help and pseudo-terminal launch/quit passed. |
| M2 | Responsive local file browser | not started | — |
| M3 | Single-file video and audio playback | not started | — |
| M4 | Playback controls and synchronized status | not started | — |
| M5 | Mixed-media queue | not started | — |
| M6 | Recovery and lifecycle reliability | not started | — |
| M7 | Complete responsive TUI | not started | — |
| M8 | Debian acceptance and usage documentation | not started | — |

## M1 — Go Foundation and CLI Contract

**Implementation decisions (2026-10-16):** Use the provisional module path `mvplayer` because no repository hosting path is defined. Bubble Tea v2.0.10 is selected, and `go mod tidy` resolved the module's Go directive to 1.26.0. The startup resolver accepts regular files and directories, resolves relative paths against the supplied working directory, verifies readable directory entries and file openability, and returns a pending file separately from its containing browse directory. `--help` and `-h` are handled before terminal startup. M1's screen states that playback is pending M3 and exits on `q` or Ctrl+C.

**Implement:** Initialize the Go module and executable at `cmd/mvplayer/`, with private application packages under `internal/`. Record the chosen module path and Go/Bubble Tea versions. Implement argument resolution for no argument, one directory, one file, and `--help`. Resolve a file into its containing browsing directory and a pending playback request; actual playback arrives in M3. Reject invalid arguments and inaccessible startup paths with useful errors. Provide a minimal TUI that exits cleanly.

**Verify:** Table-driven tests cover current-directory defaults, directory/file inputs, spaces and Unicode in paths, missing paths, excess arguments, and help. Build and launch the executable. Confirm help works without mpv or a graphical session, and quitting restores the terminal.

**Complete when:** The executable starts predictably, input resolution is tested, and temporary limitations are documented rather than reported as successful playback.

## M2 — Responsive Local File Browser

**Implement:** Add asynchronous directory listing, name sorting, hidden-file omission, recognized video/audio extensions, parent navigation, arrow and `j`/`k` movement, and current-directory filtering. Enter opens a directory or emits a file-selection action for M3. Handle empty directories and access errors in the UI. Ignore stale listing results after navigation changes.

**Verify:** Use temporary directories to test filtering, sorting, media extensions, navigation, empty results, and disappearing files. Test permission failures through a controllable filesystem seam so privileged test execution cannot hide failures. Feed delayed and out-of-order results into the UI model and confirm they cannot replace the current directory.

**Complete when:** Users can find video and audio files without blocking keyboard input, and errors leave the browser usable.

## M3 — Single-File Video and Audio Playback

**Implement:** Add a small playback adapter and one managed mpv process with JSON IPC over a private Unix socket. Connect CLI file inputs and browser selections to playback. Video opens a separate window; audio plays without a window, including audio with cover art. Starting another file replaces current playback. Include bounded startup, basic missing-mpv/load errors, process cleanup, and socket cleanup immediately.

**Verify:** Test IPC request/response correlation and interleaved events with a fake Unix-socket peer. Test application load actions through a fake playback adapter. Manually play a short video, plain audio, and audio with embedded artwork; switch between audio and video in both directions. Quit and check that the managed process and socket are gone.

**Complete when:** Both CLI and browser paths can start real audio/video, window behavior matches media type, and normal shutdown cleans up resources.

## M4 — Playback Controls and Synchronized Status

**Implement:** Add play/pause, seek, volume, stop, and video fullscreen controls. Display current item, elapsed time, duration when known, and playback state. Observe mpv changes so video-window controls update the TUI. Disable fullscreen for audio-only playback. Bind and display shortcuts, keeping browser navigation distinct from playback actions.

**Verify:** Test each action against a fake adapter, incoming state changes, unknown duration, and controls while idle. Manually pause, seek, and adjust volume from the terminal and video window; confirm matching TUI status. Check fullscreen for video and its disabled state for audio.

**Complete when:** Displayed status follows observed player state, commands remain responsive, and both control surfaces behave consistently.

## M5 — Mixed-Media Queue

**Implement:** Add an application-owned queue with enqueue, removal, selection, next/previous actions, and insertion-order playback. Enter in the browser plays immediately while preserving pending entries. Advance only on normal end-of-file. Stop at queue exhaustion and on playback failure, keeping remaining entries intact. Keep queue state in memory only.

Before implementation, record precise next/previous and current-entry removal semantics so tests and shortcut help agree. Preserve the project design's immediate-play and pending-queue behavior.

**Verify:** Test enqueue/removal, duplicates, immediate play during queued playback, next/previous boundaries, exhaustion, and failed entries. Verify that explicit stop, interrupted loading, stale events, window closure, and process failure cannot accidentally advance the queue. Manually play video → audio → video and audio → video → audio.

**Complete when:** Mixed queues advance exactly once per natural completion, preserve pending entries during immediate play, and stop predictably on errors.

## M6 — Recovery and Lifecycle Reliability

**Implement:** Complete handling for startup timeouts, command failures, IPC disconnects, unexpected player exits, and files disappearing after listing. Closing the video window stops playback while preserving the browser and queue; another selection can start playback again. Quitting or interruption shuts down the managed process, closes IPC, removes the socket, and restores the terminal. Distinguish intentional window changes during audio transitions from user closure.

**Verify:** Inject failures through fake adapters and IPC peers. Exercise repeated start/stop cycles, cancellation during startup, and shutdown with a pending command. Manually close the video window, terminate mpv externally, select another item, and interrupt MVPLAYER. Check recovery, visible errors, and resource cleanup.

**Complete when:** Every covered failure has a bounded outcome, preserves usable application state where possible, and leaves no managed process or socket after exit.

## M7 — Complete Responsive TUI

**Implement:** Finish the side-by-side browser/queue layout, Tab focus switching, status area, shortcut hints, and narrow-terminal focused-pane layout. Keep selection and focus coherent after filtering, queue changes, and resizing. Present playback and browsing errors without obscuring navigation.

**Verify:** Test focus and selection transitions and representative narrow/wide render sizes. Manually resize during playback, use both navigation styles, filter to zero matches, and inspect long filenames and error messages. Confirm controls remain discoverable and usable in each layout.

**Complete when:** The complete initial-release workflow works in both layouts without lost state or inaccessible controls.

## M8 — Debian Acceptance and Usage Documentation

**Implement:** Document installation prerequisites, build/run commands, supported launch forms, shortcuts, media behavior, and known limitations. Record the Debian, Go, mpv, and terminal versions actually verified. Address defects found during acceptance without adding deferred features.

**Verify:** On both Debian Wayland and X11 sessions, exercise CLI launch, browsing/filtering, audio/video playback, mixed queues, both control surfaces, fullscreen, resizing, errors, window closure, restart, and clean exit. Use small owned or redistributable fixtures: video with sound, plain audio, audio with artwork, and corrupt media. Record fixture provenance; keep large media outside the repository.

**Complete when:** All earlier gates pass, both display-session checks have recorded evidence, and documented setup reproduces the working application. Unavailable environments remain explicitly pending.

## Shared Verification Rules

After a Go module exists, run these from the repository root during each milestone's verification phase:

```sh
go build ./...
go test ./...
go vet ./...
```

Format changed Go files with `gofmt`. From M3 onward, also run `go test -race ./...` for process/IPC concurrency coverage where supported; record unsupported environments explicitly. Use targeted tests during iteration and the full checks at the milestone gate.

Keep default automated tests independent of installed mpv, audio devices, and graphical sessions. Keep real-player integration checks explicitly invoked and document their prerequisites and invocation when added. Fakes verify application behavior; real-player and desktop checks verify actual integration. Neither substitutes for the other.

No arbitrary coverage percentage is required. Tests must cover the milestone's behaviors and failure cases. No application tests are run as part of creating this planning document.

## Work Log

Initial entry: implementation plan created while the repository was design-only; all milestones were not started at that point.

Date / milestone / phase: 2026-10-16 / M1 / closed
Changes completed: Added the Go module, CLI, argument resolver, Bubble Tea minimal screen, and table-driven path/help/error tests. Documented M1-only limitations and dependency choices.
Decisions and rationale: Module path is `mvplayer` until a canonical remote path exists. Bubble Tea v2.0.10 is selected; `go mod tidy` sets Go 1.26.0 for the resolved dependency graph. File arguments resolve to their parent browse directory and remain pending playback until M3.
Verification commands and results: `gofmt` completed; `GOCACHE=/tmp/mvplayer-gocache go test ./...`, `GOCACHE=/tmp/mvplayer-gocache go vet ./...`, and `GOCACHE=/tmp/mvplayer-gocache go build -buildvcs=false ./...` passed. The default `go build ./...` fails only while trying to read VCS status from this workspace's placeholder `.git` directory; disabling VCS stamping builds all packages successfully. `go version` reported go1.27.1.
Manual environment and results: Built `/tmp/mvplayer`; `/tmp/mvplayer --help` printed usage and exited. Ran `/tmp/mvplayer /etc/hostname` in an 80x24 pseudo-terminal, confirmed the containing directory and pending playback file appeared, sent `q`, and observed a clean exit with terminal reset sequences. No real playback or graphical behavior is part of M1.
Failures or checks still pending: None for M1. Default VCS stamping remains unavailable in this workspace because `.git` is not a valid Git repository; `-buildvcs=false` is the verified build command here.
Milestone status and next action: complete; M1 acceptance criteria are met. Proceed to M2 when authorized.

Append one entry per implementation session using this template:

```text
Date / milestone / phase:
Changes completed:
Decisions and rationale:
Verification commands and results:
Manual environment and results:
Failures or checks still pending:
Milestone status and next action:
```
