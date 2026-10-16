# MVPLAYER — Phased Implementation Plan

## Purpose and Current State

Follow this document when implementing the design in [PROJECT-PLAN.md](PROJECT-PLAN.md). That document defines product behavior; this document defines delivery order, verification gates, and progress. Its broad first milestone is split into the eight milestones below. All eight together deliver the initial release scope.

Current state: M1–M4 are complete. The application supports CLI startup, local browsing, real audio/video playback, playback controls, and synchronized player status. M5 implementation is complete and verification is pending the user's retest of the fixes; M6–M8 are not started.

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
| M2 | Responsive local file browser | complete | `GOCACHE=/tmp/mvplayer-gocache go test ./...`, `go vet ./...`, and `go build -buildvcs=false ./...` passed; built executable launched in a pseudo-terminal, rendered entries, and quit with terminal reset. |
| M3 | Single-file video and audio playback | complete | Go tests/race/vet/build and Makefile targets passed. Under Wayland with mpv 0.40.0, video, WAV, and cover-art MP3 loaded; video→audio and audio→video TUI selections worked; shutdown left no mpv process or socket directory. User visually confirmed video window playback and audio playback without a separate media window. |
| M4 | Playback controls and synchronized status | complete | Go tests/race/vet/build and real-mpv replay/control checks passed. User confirmed first-Enter replay, seek, volume, stop, and stable TUI rendering/navigation; earlier checks confirmed pause/fullscreen and audio controls. User's acceptance of the corrected synchronized TUI closes the final pending desktop check. |
| M5 | Mixed-media queue | verification pending | Full Go tests/race/vet/build pass. Real mpv queue integration passes video → audio → video and audio → video → audio, including correct ready/Playing status. User retest of ASCII queue markers and the corrected live TUI transitions remains pending. |
| M6 | Recovery and lifecycle reliability | complete | Go tests/race/vet/build and diff checks passed; real-mpv mixed-queue and external-exit recovery integration passed. User verified window close, restart after player exit, mixed-media transitions without the earlier intermittent black screen, interruption, and cleanup. |
| M7 | Complete responsive TUI | not started | — |
| M8 | Debian acceptance and usage documentation | not started | — |

## M1 — Go Foundation and CLI Contract

**Implementation decisions (2026-10-16):** Use the provisional module path `mvplayer` because no repository hosting path is defined. Bubble Tea v2.0.10 is selected, and `go mod tidy` resolved the module's Go directive to 1.26.0. The startup resolver accepts regular files and directories, resolves relative paths against the supplied working directory, verifies readable directory entries and file openability, and returns a pending file separately from its containing browse directory. `--help` and `-h` are handled before terminal startup. M1's screen states that playback is pending M3 and exits on `q` or Ctrl+C.

**Implement:** Initialize the Go module and executable at `cmd/mvplayer/`, with private application packages under `internal/`. Record the chosen module path and Go/Bubble Tea versions. Implement argument resolution for no argument, one directory, one file, and `--help`. Resolve a file into its containing browsing directory and a pending playback request; actual playback arrives in M3. Reject invalid arguments and inaccessible startup paths with useful errors. Provide a minimal TUI that exits cleanly.

**Verify:** Table-driven tests cover current-directory defaults, directory/file inputs, spaces and Unicode in paths, missing paths, excess arguments, and help. Build and launch the executable. Confirm help works without mpv or a graphical session, and quitting restores the terminal.

**Complete when:** The executable starts predictably, input resolution is tested, and temporary limitations are documented rather than reported as successful playback.

## M2 — Responsive Local File Browser

**Implementation decisions (2026-10-16):** Start and edit filtering with `/`; Enter or Escape exits filter editing, Backspace deletes one Unicode character while filtering, and Ctrl+U clears it. Outside filter editing, Backspace navigates to the parent. Sort entries case-insensitively by name, with directories and recognized media files in the same list. Treat hidden and unsupported files as omitted. The file browser emits a typed `FileSelectedMsg` for M3 to connect to playback; until then it reports the selected path. A request counter and directory path guard discard stale asynchronous listings. A lister function seam makes directory errors deterministic in tests.

**Implement:** Add asynchronous directory listing, name sorting, hidden-file omission, recognized video/audio extensions, parent navigation, arrow and `j`/`k` movement, and current-directory filtering. Enter opens a directory or emits a file-selection action for M3. Handle empty directories and access errors in the UI. Ignore stale listing results after navigation changes.

**Verify:** Use temporary directories to test filtering, sorting, media extensions, navigation, empty results, and disappearing files. Test permission failures through a controllable filesystem seam so privileged test execution cannot hide failures. Feed delayed and out-of-order results into the UI model and confirm they cannot replace the current directory.

**Complete when:** Users can find video and audio files without blocking keyboard input, and errors leave the browser usable.

## M3 — Single-File Video and Audio Playback

**Implementation decisions (2026-10-16):** Start mpv lazily on the first playback request so browsing works without the optional player installed. Use one private mode-0700 temporary directory for the Unix socket, `--audio-display=no` to suppress cover-art windows, and mpv's normal video auto-window behavior. Bound socket startup to three seconds and each application load request to eight seconds. Immediate playback uses mpv's `loadfile ... replace` command. `make` builds the root `mvplayer` executable; `make run ARGS="..."` accepts CLI arguments and inherits caller environment such as `GOCACHE`.

**Implement:** Add a small playback adapter and one managed mpv process with JSON IPC over a private Unix socket. Connect CLI file inputs and browser selections to playback. Video opens a separate window; audio plays without a window, including audio with cover art. Starting another file replaces current playback. Include bounded startup, basic missing-mpv/load errors, process cleanup, and socket cleanup immediately.

**Verify:** Test IPC request/response correlation and interleaved events with a fake Unix-socket peer. Test application load actions through a fake playback adapter. Manually play a short video, plain audio, and audio with embedded artwork; switch between audio and video in both directions. Quit and check that the managed process and socket are gone.

**Complete when:** Both CLI and browser paths can start real audio/video, window behavior matches media type, and normal shutdown cleans up resources.

## M4 — Playback Controls and Synchronized Status

**Implementation decisions (2026-10-16):** Use Space for pause, Left/Right (or h/l) for five-second seeks, +/- for five-point volume changes, s to stop, and f to toggle fullscreen. Browser movement stays on arrows and j/k. The adapter observes mpv path, pause, time-pos, duration, volume, and fullscreen properties and emits snapshots to Bubble Tea; unknown time/duration remain omitted. Fullscreen is available only for recognized video extensions. Control IPC runs asynchronously with a three-second timeout.

**Implement:** Add play/pause, seek, volume, stop, and video fullscreen controls. Display current item, elapsed time, duration when known, and playback state. Observe mpv changes so video-window controls update the TUI. Disable fullscreen for audio-only playback. Bind and display shortcuts, keeping browser navigation distinct from playback actions.

**Verify:** Test each action against a fake adapter, incoming state changes, unknown duration, and controls while idle. Manually pause, seek, and adjust volume from the terminal and video window; confirm matching TUI status. Check fullscreen for video and its disabled state for audio.

**Complete when:** Displayed status follows observed player state, commands remain responsive, and both control surfaces behave consistently.

## M5 — Mixed-Media Queue

**Status and semantics (2026-10-16):** M4's sole remaining pending item was explicit user confirmation of video-window-to-TUI status synchronization. The latest user acceptance that the corrected TUI and controls work closes that check; the prior Go, race, vet, build, and real-mpv evidence remains recorded in the M4 log. M4 is complete. For M5, the queue contains entries in insertion order and allows duplicates; its cursor identifies the current queue item, and its resume index identifies the next pending item. Enter plays immediately and leaves pending queue entries untouched, including when queue playback is active. `a` appends the highlighted media file. `d` removes the selected pending queue entry; it does not remove or interrupt the current item. Next starts the first pending entry, or advances to the next pending entry after a queue item; it does not alter the queue on exhaustion. Previous replays the current queue item when one is active; for an immediate-play item it starts the first pending item. At the beginning or with no pending item, previous is a no-op. Natural EOF advances once to the next pending item; explicit stop, load failure, and error EOF do not advance.

**Implement:** Add an application-owned queue with enqueue, removal, selection, next/previous actions, and insertion-order playback. Enter in the browser plays immediately while preserving pending entries. Advance only on normal end-of-file. Stop at queue exhaustion and on playback failure, keeping remaining entries intact. Keep queue state in memory only.

Before implementation, record precise next/previous and current-entry removal semantics so tests and shortcut help agree. Preserve the project design's immediate-play and pending-queue behavior.

**Verify:** Test enqueue/removal, duplicates, immediate play during queued playback, next/previous boundaries, exhaustion, and failed entries. Verify that explicit stop, interrupted loading, stale events, window closure, and process failure cannot accidentally advance the queue. Manually play video → audio → video and audio → video → audio.

**Complete when:** Mixed queues advance exactly once per natural completion, preserve pending entries during immediate play, and stop predictably on errors.

## M6 — Recovery and Lifecycle Reliability

**Decisions (2026-10-16):** After an mpv exit or IPC disconnect, preserve the browser and queue and wait for the next explicit playback request to start a fresh session. Give each request an attempt ID so repeated selection of the same path cannot accept stale load results or player events. Treat matching `file-loaded` as load readiness. Bind mpv's `CLOSE_WIN` input to `stop`, leaving the managed player idle. Bubble Tea owns SIGINT/SIGTERM and terminal restoration; application shutdown cancels playback work before closing the player. Retain three-second control and eight-second load limits, and allow up to two seconds for process cleanup after the 500 ms graceful quit.

**Implement:** Complete handling for startup timeouts, command failures, IPC disconnects, unexpected player exits, and files disappearing after listing. Closing the video window stops playback while preserving the browser and queue; another selection can start playback again. Quitting or interruption shuts down the managed process, closes IPC, removes the socket, and restores the terminal. Distinguish intentional window changes during audio transitions from user closure.

**Verify:** Inject failures through fake adapters and IPC peers. Exercise repeated start/stop cycles, cancellation during startup, and shutdown with a pending command. Manually close the video window, terminate mpv externally, select another item, and interrupt MVPLAYER. Check recovery, visible errors, and resource cleanup.

**Complete when:** Every covered failure has a bounded outcome, preserves usable application state where possible, and leaves no managed process or socket after exit.

**Implementation log (2026-10-16):** Added attempt IDs to loads and playback snapshots, wait-for-`file-loaded` load completion, matching error-EOF details, process-exit monitoring and reaping, command write deadlines, session invalidation on IPC closure, and retry on the next explicit request only. Added `CLOSE_WIN stop`, application-context cancellation, visible playback failures, browser/queue preservation, and regression tests for repeated-path attempts, stale events, failure state, and window binding. README now describes recovery and window-close behavior.

**Verification:** `GOCACHE=/tmp/mvplayer-gocache go test ./...`, `GOCACHE=/tmp/mvplayer-gocache go test -race ./...`, `GOCACHE=/tmp/mvplayer-gocache go vet ./...`, `GOCACHE=/tmp/mvplayer-gocache go build -buildvcs=false ./...`, and `git diff --check` passed. Outside the sandbox, the opt-in real-mpv mixed-media queue test passed all sequences, and the lifecycle integration passed external mpv termination followed by an explicit successful reload. The user manually confirmed that closing the video window leaves MVPLAYER usable and playback can be restarted; externally terminated mpv reports a clear error and can also be restarted; mixed-media transitions work and the previously intermittent black screen did not recur; interruption restores the terminal and cleans up the player/socket. M6 is complete. M5's separate queue marker and Loading-state retest remains pending.

**Closure log (2026-10-16):** Reclassified normal mpv `quit` after video-window closure as Stopped, replaced raw process/IPC exit details with an actionable recovery message, and waited for mpv's final event before classifying process exit. The user's final retest confirmed the error is clear and the black screen did not recur.

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

Date / milestone / phase: 2026-10-16 / M4 / verification pending
Changes completed: Added mpv commands for pause, relative seek, volume, stop, and fullscreen. The adapter observes path, pause, playback time, duration, volume, and fullscreen and publishes state snapshots. The TUI handles these updates asynchronously, renders playback status, exposes controls while media is active, and gates fullscreen to recognized video files. Added TUI and IPC event tests and documented shortcuts in README.
Decisions and rationale: Space toggles pause; Left/Right or h/l seek five seconds; +/- changes volume by five percentage points, bounded to 0–100; s stops; f toggles fullscreen for video. Commands have three-second contexts. Browser movement remains unchanged. Unknown time and duration are omitted from the status line.
Verification commands and results: `GOCACHE=/tmp/mvplayer-gocache go test ./...`, `GOCACHE=/tmp/mvplayer-gocache go test -race ./...`, `GOCACHE=/tmp/mvplayer-gocache go vet ./...`, `GOCACHE=/tmp/mvplayer-gocache go build -buildvcs=false ./...`, and `git diff --check` passed. Fake-player tests cover command mapping, volume bounds via action logic, audio fullscreen gating, and incoming status; IPC test covers property events interleaved with responses.
Manual environment and results: mpv v0.40.0 and a Wayland display are available, but interactive terminal/video-window controls have not been manually exercised during this implementation session. X11 is also unverified.
Failures or checks still pending: Manually pause, seek, adjust volume, and fullscreen from terminal and video window; confirm video-window changes update TUI and confirm fullscreen is disabled for audio. Debian Wayland/X11 acceptance remains for M8.
Milestone status and next action: verification pending the manual control checks; M4 implementation and automated checks are complete.

Date / milestone / phase: 2026-10-16 / M4 / user desktop control verification
Changes completed: Recorded user-observed control behavior for video and audio playback.
Decisions and rationale: Treat controls as verified only where the user confirmed they respond in the desktop session; retain M4 as pending while required video controls fail.
Verification commands and results: User manually confirmed video fullscreen and pause/resume work. Video seeking, volume, and stop are unresponsive. For audio, pause/resume, seeking, volume, and stop work; fullscreen is disabled and has no effect as intended.
Manual environment and results: User-tested in the interactive terminal and media window. Video-window-to-TUI status synchronization was not reported.
Failures or checks still pending: Diagnose and fix video seeking, volume, and stop. Confirm video-window control changes update TUI status. M8 Debian Wayland/X11 acceptance remains pending.
Milestone status and next action: M4 remains verification pending because three video controls are unresponsive; investigate the video-only control path.

Date / milestone / phase: 2026-10-16 / playback IPC recovery / closed
Changes completed: Fixed repeat-load failures after mpv's IPC connection closes. The adapter now recognizes a disconnected reader or broken-pipe write, discards the dead process/socket session, starts a fresh managed mpv, and retries the requested load once. Concurrent load requests are serialized so recovery cannot tear down a replacement session started by another load. Added a broken-pipe regression check.
Decisions and rationale: The reported error is a stale-socket transport failure. A non-nil `net.Conn` does not prove mpv is still connected; recovery is limited to IPC disconnects so normal mpv command errors are still surfaced to the user.
Verification commands and results: `GOCACHE=/tmp/mvplayer-gocache go test ./internal/playback -run 'TestCommand(ClassifiesBrokenPipe|CorrelatesResponse)' -count=1`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build -buildvcs=false ./...`, and `git diff --check` passed.
Manual environment and results: Not manually replayed through the graphical player in this session; the broken-pipe code path was exercised with a closed `net.Pipe` peer.
Failures or checks still pending: Confirm repeated Enter playback and recovery after externally closing mpv in the user's desktop session.
Milestone status and next action: fix implemented and automated checks pass; verify repeated playback in the available Wayland/X11 acceptance sessions.

Date / milestone / phase: 2026-10-16 / EOF reload state / closed
Changes completed: Prevented a delayed empty-path snapshot from the previous EOF from clearing a newly selected file while its playback is starting. The TUI retains the requested item until mpv reports a time position for it, then applies later EOF state normally. Added a regression test for the stale-state ordering. Added an opt-in headless real-mpv integration check that reloads a short WAV three times after natural EOF.
Decisions and rationale: A synthetic delayed EOF snapshot made the TUI regression fail before the fix and pass afterward. Headless mpv testing showed repeated IPC load requests themselves succeed after EOF, so the fix is in TUI state reconciliation rather than another player restart/retry.
Verification commands and results: The opt-in `MVPLAYER_MPV_INTEGRATION=1 GOCACHE=/tmp/mvplayer-gocache go test ./internal/playback -run TestMPVReloadsSameFileAfterEOFIntegration -count=1 -v` passed against real mpv, including three reloads after natural EOF. `GOCACHE=/tmp/mvplayer-gocache go test ./...`, `go test -race ./...`, `go vet ./...`, `go build -buildvcs=false ./...`, and `git diff --check` passed.
Manual environment and results: Headless real-mpv IPC behavior verified with mpv v0.40.0; graphical TUI replay of the reported MP4 sequence was not performed in this session.
Failures or checks still pending: Confirm the user's visual/audio MP4 replay now starts on the first Enter. M4 Wayland/X11 desktop controls remain pending.
Milestone status and next action: EOF reload state fix and automated checks are complete; verify the original graphical MP4 report in the user's session.

Date / milestone / phase: 2026-10-16 / playback selection indicator / verification pending
Changes completed: Cleared the persistent `Selected:` line when the browser selection is handed to playback. It now gives way to the `Now playing:` status instead of remaining as a stale indicator. Added a regression assertion that failed before the change.
Decisions and rationale: The label was assigned on Enter but had no clearing path, so it remained visible after playback was requested and obscured whether the selection transitioned. `make run` invokes `go run`, which builds the current source and uses the content-addressed Go build cache; it does not launch the root `mvplayer` binary.
Verification commands and results: Exact-MP4 headless and normal-video-output integration checks passed through the playback adapter and TUI model after EOF. The `Selected` regression failed before the fix and passed after it. `GOCACHE=/tmp/mvplayer-gocache go test ./...`, `go test -race ./...`, `go vet ./...`, `go build -buildvcs=false ./...`, and `git diff --check` passed.
Manual environment and results: The actual `make run` terminal interaction has not yielded a useful captured screen trace; the integration test reproduces its Enter-to-selection-to-load sequence using the TUI model and real mpv.
Failures or checks still pending: User retest of the latest source is needed to confirm the status transition and first-Enter playback in their terminal.
Milestone status and next action: stale indicator fix complete; await user's retest of the current source.

Date / milestone / phase: 2026-10-16 / M1 / closed
Changes completed: Added the Go module, CLI, argument resolver, Bubble Tea minimal screen, and table-driven path/help/error tests. Documented M1-only limitations and dependency choices.
Decisions and rationale: Module path is `mvplayer` until a canonical remote path exists. Bubble Tea v2.0.10 is selected; `go mod tidy` sets Go 1.26.0 for the resolved dependency graph. File arguments resolve to their parent browse directory and remain pending playback until M3.
Verification commands and results: `gofmt` completed; `GOCACHE=/tmp/mvplayer-gocache go test ./...`, `GOCACHE=/tmp/mvplayer-gocache go vet ./...`, and `GOCACHE=/tmp/mvplayer-gocache go build -buildvcs=false ./...` passed. The default `go build ./...` fails only while trying to read VCS status from this workspace's placeholder `.git` directory; disabling VCS stamping builds all packages successfully. `go version` reported go1.27.1.
Manual environment and results: Built `/tmp/mvplayer`; `/tmp/mvplayer --help` printed usage and exited. Ran `/tmp/mvplayer /etc/hostname` in an 80x24 pseudo-terminal, confirmed the containing directory and pending playback file appeared, sent `q`, and observed a clean exit with terminal reset sequences. No real playback or graphical behavior is part of M1.
Failures or checks still pending: None for M1. Default VCS stamping remains unavailable in this workspace because `.git` is not a valid Git repository; `-buildvcs=false` is the verified build command here.
Milestone status and next action: complete; M1 acceptance criteria are met. Proceed to M2 when authorized.

Date / milestone / phase: 2026-10-16 / M2 / closed
Changes completed: Added asynchronous local directory listing with hidden/unsupported-file omission, case-insensitive name sorting, video/audio extension recognition, parent navigation, movement by arrows or j/k, `/` filtering, empty/error states, stale-result rejection, and typed file-selection messages. Preserved M1's pending CLI file display. Added browser and TUI tests for listing/filtering, media classification, missing directories, selection, error recovery, navigation, and stale responses.
Decisions and rationale: Filtering uses `/`, Backspace navigates to the parent outside filter editing, Enter/Escape ends filter editing, and Ctrl+U clears the query. The M3 playback connection is represented by `FileSelectedMsg` so file selection has a stable event boundary.
Verification commands and results: `GOCACHE=/tmp/mvplayer-gocache go test ./...`, `GOCACHE=/tmp/mvplayer-gocache go vet ./...`, and `GOCACHE=/tmp/mvplayer-gocache go build -buildvcs=false ./...` passed. The default Go cache was not writable, so verification used `/tmp`; default VCS stamping remains unavailable in the placeholder Git workspace.
Manual environment and results: Built `/tmp/mvplayer`, launched it against the repository in a pseudo-terminal, confirmed directories rendered, pressed `q`, and observed the terminal reset.
Failures or checks still pending: No permission-denied OS test was used because privileged execution can bypass it; the injected lister error test covers the UI failure path. No separate desktop or mpv checks apply to M2.
Milestone status and next action: complete; proceed to M3 when authorized.

Date / milestone / phase: 2026-10-16 / M3 / verification pending
Changes completed: Added a managed mpv adapter using JSON IPC over a private Unix socket, asynchronous browser and CLI file loads, visible load errors, audio cover-art window suppression, process/socket cleanup, and root Makefile targets for default build, run, and test. Updated README usage.
Decisions and rationale: mpv starts lazily so users can still browse without the runtime dependency. Playback requests replace current playback; adapter startup waits up to three seconds and a load request is bounded to eight seconds. `make run ARGS="..."` passes CLI arguments and inherits environment variables.
Verification commands and results: `GOCACHE=/tmp/mvplayer-gocache go test ./...`, `GOCACHE=/tmp/mvplayer-gocache go test -race ./...`, `GOCACHE=/tmp/mvplayer-gocache go vet ./...`, `GOCACHE=/tmp/mvplayer-gocache go build -buildvcs=false ./...`, `GOCACHE=/tmp/mvplayer-gocache make`, `GOCACHE=/tmp/mvplayer-gocache make test`, and `GOCACHE=/tmp/mvplayer-gocache make run ARGS=--help` passed. Playback command response correlation/interleaved event handling is covered using an in-memory stream peer; Unix sockets are denied by the current sandbox.
Manual environment and results: Real mpv playback, audio/video transitions, process cleanup, and Wayland/X11 window behavior were not verified in this environment.
Failures or checks still pending: Unix-socket peer verification could not bind a Unix socket (`setsockopt: operation not permitted`) in the sandbox. Confirm real mpv behavior and cleanup in a Debian desktop session.
Milestone status and next action: verification pending; continue M3 acceptance checks when an mpv-capable desktop environment is available.

Date / milestone / phase: 2026-10-16 / M3 / verification update
Changes completed: Confirmed mpv v0.40.0 is installed. Generated short video-with-audio, WAV, and MP3-with-embedded-cover-art fixtures under `/tmp` using ffmpeg. Launched MVPLAYER in a pseudo-terminal under the available Wayland display for each CLI file form, selected files through the browser for video→audio and audio→video transitions, and quit each session.
Decisions and rationale: Fixtures remain outside the repository because they are temporary synthetic acceptance media.
Verification commands and results: Re-ran `GOCACHE=/tmp/mvplayer-gocache go test ./...`, `go test -race ./...`, `go vet ./...`, and `make`; all passed. After each player session, checked that no `/tmp/mvplayer-mpv-*` socket directory or mpv process remained. The TUI displayed each requested path as playing and exited with terminal restoration.
Manual environment and results: mpv v0.40.0 at `/usr/bin/mpv`; `WAYLAND_DISPLAY=wayland-0` and `DISPLAY=:0`. Video, audio-only WAV, and MP3 with attached cover art loaded. Both audio/video selection directions loaded successfully. Direct visual confirmation that video opens a window and audio (including cover art) does not remains unobserved from this interface.
Failures or checks still pending: Visual window visibility and fullscreen/window-close behavior remain unverified; fullscreen/window-close are later milestones. Debian Wayland/X11 acceptance across both session types is M8.
Milestone status and next action: verification pending direct observation of the video/audio window behavior; continue with the remaining M3 visual check before M4.

Date / milestone / phase: 2026-10-16 / M3 / closed
Changes completed: Recorded user-confirmed visual acceptance for separate video-window playback and audio-only playback without a separate media window. The cover-art MP3 was used for the audio check; quitting with `q` succeeded. Combined with the earlier synthetic fixture checks for CLI loading, browser selection, both video/audio selection directions, and process/socket cleanup, all M3 criteria are met.
Decisions and rationale: The user's desktop observation supplies the visual evidence unavailable through the agent's terminal view.
Verification commands and results: Prior Go tests, race tests, vet, build, and Makefile target checks passed as recorded above. User ran `make run ARGS=/tmp/mvplayer-check-video.mp4` and `make run ARGS=/tmp/mvplayer-check-cover.mp3`, confirming visible video playback and audible cover-art audio without a separate media window.
Manual environment and results: User reported video displayed in a separate window and played; `q` exited; cover-art audio played without a separate window. User also confirmed selecting video from the browser opened the video window and played.
Failures or checks still pending: None for M3. Full Debian X11 and Wayland acceptance across controls, fullscreen, window closure, and recovery remains scheduled for M8.
Milestone status and next action: complete; M3 acceptance criteria are met. M4 is the next milestone.

Date / milestone / phase: 2026-10-16 / M4 playback status at EOF / verification pending
Changes completed: Added mpv end-file state handling and TUI labels for ended, stopped, and failed playback. Preserved the completed filename when mpv clears its path immediately after EOF, so the status row continues to show Ended rather than reverting to Playing or disappearing.
Decisions and rationale: The playback adapter reports mpv's end-file event, while the TUI preserves the item across the subsequent empty-path property snapshot. This matches mpv's event ordering and keeps EOF visible until the user selects another file.
Verification commands and results: `go test ./...` passed. The opt-in real-mpv TUI replay check passed headlessly against `/tmp/mvplayer-check-video.mp4`; one video-output integration run did not receive a start event, while the headless run passed.
Manual environment and results: The user's reported screen state is not available for direct observation in this session.
Failures or checks still pending: Retest EOF status in the user's `make run` terminal. M4 desktop controls and fullscreen acceptance remain pending.
Milestone status and next action: EOF status fix implemented; user should confirm the `Ended` label after playback finishes.

Date / milestone / phase: 2026-10-16 / M4 repeat-play status / verification pending
Changes completed: Clear the prior playback's ended/error flag and stale time values when a new file selection starts, and again after the load request succeeds. This prevents the previous attempt's Failed/Ended state from contaminating the newly selected attempt.
Decisions and rationale: Real mpv reloads the same MP4 after natural EOF through the adapter and TUI model. The remaining `Failed` transition can arise from carrying the old status into the new Loading attempt, so the TUI now starts each selection with fresh playback status.
Verification commands and results: `GOCACHE=/tmp/mvplayer-gocache go test ./...` passed. Headless exact-MP4 `TestEnterReloadsAfterEOFIntegration` passed. Repeated runs with normal video output are intermittent: some did not report initial advancing time within eight seconds.
Manual environment and results: User retested with `make run`: after the first playback reaches Ended, pressing Enter again briefly shows Loading and then Failed. There is no `Error:` line. Resetting stale TUI status did not resolve the issue.
Known bug (deferred): Replaying the same media file with Enter after natural EOF transitions from Loading to Failed, despite no visible error message. Real mpv adapter and headless TUI integration checks reload the exact MP4 successfully, so the remaining issue is specific to the user's interactive playback/session path and remains undiagnosed. Revisit after the main project work is complete; capture mpv/TUI event ordering and process/window state during the second Enter.
Failures or checks still pending: Same-file replay after EOF remains broken in the user's interactive session. M4 desktop controls and fullscreen acceptance remain pending.
Milestone status and next action: EOF display fix is verified by the user. Defer repeat-play investigation until the main project work is complete.

Date / milestone / phase: 2026-10-16 / video replay and controls / fixed
Changes completed: Completed the private mpv window bindings for pause, precise five-second seeking, volume, stop, and fullscreen. Enabled software GPU rendering as a fallback. Classified reader-side IPC disconnects as recoverable so the first Load retries automatically, and cleared the current path when mpv reports it unavailable without a data field. Preserved the existing working-tree changes.
Decisions and rationale: Reproduced a real video stop/reload failure: loadfile succeeded but mpv emitted end-file/error, with file seek failures and partial-file errors. Allowing software GPU rendering removed that failure. The mpv 0.40 DRM initialization path leaves its descriptor zero-initialized when device discovery fails, then unconditionally closes it during cleanup; a later media load can reuse descriptor 0. Source: https://github.com/mpv-player/mpv/blob/v0.40.0/video/out/drm_common.c (vo_drm_init and vo_drm_uninit). Separately reproduced an IPC disconnect after a successful command write: the generic response error bypassed the existing retry, requiring another Enter. Transport errors now retain their error identity. Window keypress acknowledgements precede the effects, so integration checks wait for observed property changes rather than reading immediately. Precise seeks avoid snapping back to the same keyframe.
Verification commands and results: The new read-disconnect command and first-Load recovery regressions failed before the fix and passed afterward. Real-mpv video checks verified +, =, -, Left/Right, h/l, Space, s, and reload after stop using a twenty-second synthetic MP4. The TUI integration verified six consecutive Enter loads after natural EOF with video output enabled. Full suite passed with MVPLAYER_MPV_INTEGRATION=1, MVPLAYER_MPV_FIXTURE=/tmp/mvplayer-check-video.mp4, MVPLAYER_MPV_CONTROLS_FIXTURE=/tmp/mvplayer-controls-video.mp4, and MVPLAYER_MPV_VIDEO_OUTPUT=1. GOCACHE=/tmp/mvplayer-gocache go test -race ./..., go vet ./..., and go build -buildvcs=false -o /tmp/mvplayer-fixed ./cmd/mvplayer passed. git diff --check passed.
Manual environment and results: mpv v0.40.0, available Wayland desktop and software renderer. Checks exercised mpv's actual window input handler through IPC keypress; physical desktop key presses and audible output were not observed. Synthetic media remained under /tmp.
Failures or checks still pending: No reproduced replay/control failures remain. M4 physical-key and fullscreen acceptance and M8 Debian Wayland/X11 acceptance remain pending. This entry supersedes the earlier deferred repeat-play bug status.
Milestone status and next action: Reported video bugs fixed with regression coverage; M4 remains pending the remaining desktop acceptance checks.

Date / milestone / phase: 2026-10-16 / TUI corruption from playback diagnostics / fixed
Changes completed: Stopped forwarding mpv stderr to the application's terminal. Added a subprocess regression that launches a fake mpv emitting stdout/stderr and asserts neither reaches the application output. Recorded user acceptance of first-Enter video replay, seeking, volume, and stop.
Decisions and rationale: Audio backends can write directly to stderr despite --no-terminal. Those writes bypass Bubble Tea and move the terminal cursor, corrupting the status/menu rendering and subsequent navigation. Discard child stderr so Bubble Tea owns terminal output; explicit mpv log-file diagnostics remain available for integration failures.
Verification commands and results: TestMPVKeepsChildOutputOffTerminal failed before the fix with backend warning on stderr, then passed in go test ./.... go vet ./... and go build -buildvcs=false -o /tmp/mvplayer-fixed ./cmd/mvplayer passed, using GOCACHE=/tmp/mvplayer-gocache. Real-video TestMPVWindowBindingsIntegration passed for volume, seeking, pause, stop, and reload without backend warnings reaching the terminal. git diff --check passed.
Manual environment and results: User confirmed replay and the previously broken video controls work, and supplied PipeWire stderr output interleaved with the TUI as evidence of display corruption. The terminal-output regression runs unattended without a display, media fixture, or IPC socket.
Failures or checks still pending: No automated failures remain. Physical observation of the corrected TUI after restarting the application and M8 Debian Wayland/X11 acceptance remain pending.
Milestone status and next action: Terminal-output corruption fixed; restart the application to use the updated child-process output routing.

Date / milestone / phase: 2026-10-16 / playback and TUI fixes / user accepted
Changes completed: Recorded the user's confirmation that video replay starts on the first Enter, seek/volume/stop respond, and TUI rendering/navigation work after suppressing backend stderr. Updated the current-state summary and M4 verification evidence. Prepared the playback fixes, regression checks, usage documentation, and existing synthetic-video screenshots for one commit.
Decisions and rationale: The user's desktop retest closes the reported playback and TUI defects and supersedes the earlier requests to restart/retest. M4 remains verification pending only for explicit confirmation that video-window changes appear in the TUI; preserve earlier evidence and leave M5–M8 unchanged.
Verification commands and results: Used the passing Go tests, race checks, vet, build, and real-mpv checks recorded in the preceding entries. User reported that the corrected application now works. git diff --check passed before committing; no source changes were made during this documentation/commit session.
Manual environment and results: User confirmed playback controls and the corrected TUI in the interactive desktop session. Existing mpv-shot0001.jpg through mpv-shot0004.jpg show the synthetic video test pattern captured during the earlier control investigation.
Failures or checks still pending: No reported playback or TUI defect remains. Explicit video-window-to-TUI status synchronization verification remains for M4; full Debian Wayland/X11 acceptance remains for M8.
Milestone status and next action: Reported fixes accepted and ready to commit; finish the remaining M4 verification before starting M5.

Date / milestone / phase: 2026-10-16 / M4 / closed
Changes completed: Closed M4 after the user confirmed that the corrected TUI and playback controls work. Updated the tracker and current-state summary.
Decisions and rationale: The only prior pending item was explicit confirmation that video-window control changes synchronize to the TUI. The user's latest acceptance of the corrected synchronized TUI closes that check. The earlier manual report had not explicitly named the status row, which is why the tracker remained pending despite control fixes and tests.
Verification commands and results: Prior M4 Go tests, race checks, vet, build, and real-mpv replay/control checks remain recorded in the preceding work-log entries.
Manual environment and results: User accepted the corrected playback controls and stable TUI rendering/navigation after restarting with the stderr routing fix.
Failures or checks still pending: None for M4. Full Debian Wayland/X11 acceptance remains in M8.
Milestone status and next action: M4 complete; M5 preparation and implementation started.

Date / milestone / phase: 2026-10-16 / M5 / implementation
Changes completed: Recorded M5 queue semantics and added an application-owned insertion-order queue with duplicate support, append/removal, queue selection, next/previous actions, immediate-play preservation, and natural-EOF advancement. Added queue rendering/shortcut help and README instructions. Added focused tests for queue advancement, stale EOF events, immediate play, enqueue/removal, and failed loads.
Decisions and rationale: Pending queue items remain in the queue after playback; the current queue index distinguishes played/current entries from removable pending entries. `a` enqueues, `[`/`]` select a queue row, `d` removes a pending row, `n` advances, and `p` replays the active queued item or starts the first pending item. Browser Enter preserves all pending entries. Only an observed natural EOF on an active queue item advances automatically.
Verification commands and results: `gofmt -w internal/tui/model.go internal/tui/model_test.go` and `GOCACHE=/tmp/mvplayer-gocache go test ./...` passed.
Manual environment and results: Not performed for this implementation pass.
Failures or checks still pending: Shared race/vet/build checks and M5 verification of stop, exhaustion, boundary behavior, and mixed real-media playback remain. Queue selection uses dedicated `[`/`]` keys pending the full focus/layout work in M7.
Milestone status and next action: M5 in progress; continue implementation and verification.

Date / milestone / phase: 2026-10-16 / M5 / implementation update
Changes completed: Tracked the resume position across immediate playback so `n` resumes at the next pending queue entry instead of replaying the already-played prefix. Added coverage for previous/next boundaries, immediate-play resume, exhaustion, and re-playing a prior queued item.
Decisions and rationale: Queue history remains visible in insertion order; the resume index identifies the pending suffix. Removing a row before the resume point adjusts the index, and past/current entries cannot be removed as pending entries.
Verification commands and results: `gofmt -w internal/tui/model.go internal/tui/model_test.go`, `GOCACHE=/tmp/mvplayer-gocache go test ./...`, `GOCACHE=/tmp/mvplayer-gocache go test -race ./...`, `GOCACHE=/tmp/mvplayer-gocache go vet ./...`, `GOCACHE=/tmp/mvplayer-gocache go build -buildvcs=false ./...`, and `git diff --check` passed.
Manual environment and results: Not performed for queue playback in this update.
Failures or checks still pending: Verify explicit stop and failed media do not advance, queue exhaustion in the TUI, and real video → audio → video and audio → video → audio playback. Debian Wayland/X11 acceptance remains in M8.
Milestone status and next action: M5 remains in progress; complete the outstanding verification and refine behavior if needed.

Date / milestone / phase: 2026-10-16 / M5 / bug fixes and verification pending
Changes completed: Added an explicit `Ready` playback state driven by mpv's `file-loaded` event, so queued items leave Loading even when `time-pos` is not yet available. Queue state resets readiness on each new load, and natural EOF advancement waits until the current item is ready. Fixed the listener chain so an EOF transition batches the next queue load and the next player-event listener; the missing listener was the cause of the persistent Loading state and stalled queue. Replaced the unsupported Unicode current-item marker with ASCII `*`, keeping `>` for the selected row and `>*` when both refer to one row. Added regression coverage for readiness without time-pos, listener continuity after EOF, playback failure without auto-advance, queue markers, and real mixed-media queue transitions. Addressed the request to use a fake-player failure case where no corrupt fixture was available.
Decisions and rationale: Playback readiness is an mpv lifecycle event, not an inference from time-position availability. Requiring readiness before auto-advance prevents stale end-file snapshots from skipping a just-requested next item. ASCII markers render on terminals without the relevant Unicode glyph support.
Verification commands and results: Readiness, marker, and listener-continuity regressions were confirmed failing before their fixes. `GOCACHE=/tmp/mvplayer-gocache go test ./...`, `GOCACHE=/tmp/mvplayer-gocache go test -race ./...`, `GOCACHE=/tmp/mvplayer-gocache go vet ./...`, `GOCACHE=/tmp/mvplayer-gocache go build -buildvcs=false ./...`, and `git diff --check` passed. With Unix socket access, `env MVPLAYER_MPV_QUEUE_INTEGRATION=1 GOCACHE=/tmp/mvplayer-gocache go test ./internal/tui -run '^TestMixedMediaQueueIntegration$' -count=1 -v` passed video → audio → video, audio → video → audio, and audio → audio → audio through the same player-listener command flow used by Bubble Tea. The sandbox denied mpv's Unix socket, so the integration command was rerun outside it.
Manual environment and results: User's original M5 findings are preserved in `M5-VERIFICATIONS.md`; no post-fix physical-terminal retest has been reported yet.
Failures or checks still pending: User should retest queue marker rendering, Loading-to-Playing status, both mixed-media orders, and natural EOF advancement in the interactive terminal. Stop, exhaustion, immediate-play preservation, and failed-playback behavior were reported working or covered by existing/focused tests.
Milestone status and next action: M5 verification pending the user's desktop retest.

Date / milestone / phase: 2026-10-16 / M5 / queue listener regression fix
Changes completed: Fixed the remaining reported Loading state and stalled queue. Natural EOF handling had returned only the next item's load command, replacing the one-shot Bubble Tea player listener. It now schedules the load and next listener together. Added a command-boundary regression that failed before the fix because EOF returned `loadResult` instead of a batch, then verified the next `Ready` event is consumed.
Decisions and rationale: Each playback event listener consumes exactly one state message and must be rescheduled for the next one. Queue advancement is the only branch that had returned a load command instead of the listener, so it silently stopped observing subsequent mpv state.
Verification commands and results: `GOCACHE=/tmp/mvplayer-gocache go test ./...`, `GOCACHE=/tmp/mvplayer-gocache go test -race ./...`, `GOCACHE=/tmp/mvplayer-gocache go vet ./...`, `GOCACHE=/tmp/mvplayer-gocache go build -buildvcs=false ./...`, and `git diff --check` passed. The opt-in real-mpv test passed all three queue sequences, driven through the Bubble Tea command/listener flow.
Manual environment and results: Awaiting the user's retest in the interactive terminal.
Failures or checks still pending: Confirm in the TUI that each queue transition leaves Loading, updates the current row, and advances to the third item in the reported sequences.
Milestone status and next action: M5 remains verification pending; user retest the updated build.

Date / milestone / phase: 2026-10-16 / pre-M5 manual verification setup
Changes completed: Configured the TUI to clear the screen on startup and render in Bubble Tea's alternate screen so it uses the full terminal window. Moved the downloaded video and audio fixtures into the repository-root `testdata/` directory and added `/testdata/` to `.gitignore`.
Decisions and rationale: The startup clear is sent through Bubble Tea's `ClearScreen` message from `Init`, keeping terminal writes under the renderer. The alternate-screen view provides full-window display and automatic terminal restoration when the application exits. The user requested that fixtures remain local and ignored by Git.
Verification commands and results: Ran `gofmt` and `git diff --check`; tests were not run so the user can continue with the requested verification session.
Manual environment and results: Confirmed both fixture files are present in `testdata/` with `ls -lh`; confirmed Git reports the directory as ignored.
Failures or checks still pending: User will continue M5 verification with these fixtures. Startup full-screen behavior has not yet been manually observed.
Milestone status and next action: M5 remains in progress; user can now run the planned playback checks using `testdata/mvplayer-sample-video.webm` and `testdata/mvplayer-sample-audio.wav`.

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
