# MVP-3: Selected file information

- Status: verified
- Implementation approved: yes
- Approved by/date: User / 2026-10-18
- Branch: feature/mvp-3-selected-file-information
- PR: https://github.com/castevet6/mvplayer/pull/4
- Verification: automated checks pass; user confirms interactive retest

## Problem and intended outcome

The browser shows playable filenames and, in recursive mode, relative paths.
Inspecting one file currently requires leaving MVPLAYER. Let the user press
`i` on a browser media row to inspect that file and return to the same browsing
position without starting playback or changing the queue.

The user selected a dismissible overlay and requested media information beyond
filesystem statistics, including duration, codecs, and resolution. This plan
includes those choices and proposes the probing and failure behavior below.
Implementation approval remains pending.

## Scope and boundaries

### Selected interaction

- Press `i` while Browser has focus and a recognized media row is selected to
  open a dismissible file-information modal. Use the row's absolute `Path`,
  including in recursive and filtered listings, rather than its display name.
- While editing a filter, `i` remains search text. While help is open, it does
  not open information. Queue focus, folder rows, empty results, and a loading
  browser have no file-information action. Queue inspection is deferred.
- Use a centered panel over the main view where space permits. At narrow
  widths, use the available screen for the same modal content. Keep a visible
  title and dismissal/scroll hints. Retain the existing resize guidance below
  the application's minimum supported size of 24 columns by 10 rows.
- `Escape`, `i`, or `Enter` dismisses the modal. Up/Down or `j`/`k` scroll
  details; PageUp/PageDown scroll a page. `q` and Ctrl+C keep their quit
  behavior. Other keys do not navigate, enqueue, start playback, edit filters,
  or open help while the modal is open.
- Playback events, queue advancement after natural EOF, and resizing continue
  to be processed. Modal scrolling and dismissal preserve browser directory,
  mode, query, cursor, scroll position, and focus. Existing playback continues;
  terminal playback shortcuts resume after dismissal. Video-window controls
  retain their existing behavior.
- Show a loading state immediately, then filesystem details while media probing
  continues. Add media results or an actionable probe warning inside the modal.
  Closing a loading modal and opening another file must not allow an earlier
  result to replace new content or reopen the old modal.

### Information to display

Use Go's standard library for filesystem details and a separate `ffprobe`
process for media information:

| Field | Display |
| --- | --- |
| Name | Selected path's basename, including extension |
| Path | Absolute selected path, wrapped and scrollable |
| Size | Human-readable binary units plus exact byte count |
| Modified | Local date/time with numeric timezone offset |
| Container | Reported format name |
| Duration | Container duration as hours/minutes/seconds; `Unknown` if absent or invalid |
| Overall bitrate | Reported container bitrate in readable units, or `Unknown` |
| Video streams | Stream index, codec, encoded width × height, average frame rate, and reported bitrate |
| Audio streams | Stream index, codec, sample rate, channel count/layout, and reported bitrate |

List every video and audio stream in stream-index order within its section so
multiple tracks are represented. Show language and title when reported for those
streams. Label attached pictures separately as cover art; their dimensions must
not appear as the movie's resolution or cause audio-only files to look like video.
Show `None` for absent video/audio sections. Missing or invalid individual values
are `Unknown`, including `N/A`, zero-denominator frame rates, and nonfinite or
negative numeric values. Use reported average frame rate and label it as such;
encoded dimensions do not imply display dimensions after rotation or pixel-aspect
correction. Do not infer missing bitrate or duration by scanning the whole file.
Do not substitute current playback metadata for the selected file's information.

Read metadata on each opening, rather than trusting directory-list snapshots or
caching between openings. Follow media-file symlinks for size and modification
time while displaying the selected link path; show an error for a broken link.
If a stale row now refers to a directory or another nonregular filesystem item,
show an error instead of presenting it as a media file. Preserve zero-byte file
details. Recognized extensions identify browser eligibility, not proven media
content or codec support.

Sanitize filesystem text before styling, wrap long paths without losing their
content, and keep output within the terminal dimensions, including Unicode
names and `NO_COLOR`. Clamp scrolling after resizing and keep dismissal hints
visible.

General tag browsing, subtitle/attachment details beyond identifying cover art,
artwork rendering, track selection, filesystem watching, persistent caches,
directory inspection, and queue inspection are deferred. Language/title on
video/audio streams are included; no new Go dependency is proposed.

### Probing and failure behavior

- Introduce `internal/mediainfo` to own filesystem inspection, probe execution,
  JSON parsing, and normalized results. Keep this independent of the playback
  adapter and TUI formatting. It must never load the selected file into the
  managed mpv process or create a playback window.
- Discover `ffprobe` on PATH when inspection is requested. It is an additional
  executable dependency for media details, not for startup, browsing, or playback.
  On Debian, document `sudo apt install ffmpeg`. If absent, keep filesystem
  details visible and show that command in the modal.
- Run an argument-vector command without a shell, using the absolute selected
  path as the explicit input argument. Request JSON with a restricted
  `-show_entries` field list for format, streams, dispositions, and language/title.
  Do not enumerate packets/frames or decode the entire file. Restrict probing to
  local file access; network references embedded in a file must not initiate
  network access. Unsupported inputs produce an inspection warning.
- Apply a 10-second probe timeout. Cancel on dismissal, replacement, and shutdown;
  terminate and reap the child process. Bound stdout to 1 MiB and diagnostic
  stderr to 16 KiB; exceeding stdout fails the probe rather than parsing truncated
  JSON. Sanitize any diagnostic displayed in the modal.
- Missing/unreadable files and nonregular targets show a file error and skip
  probing. A zero-byte file still shows its filesystem details with a media
  inspection warning. On timeout, nonzero exit, malformed JSON, or output-limit
  failure, retain filesystem details and show the reason; do not present media
  fields as successfully inspected. Valid JSON with missing fields is a partial
  success, with unavailable values explicitly marked `Unknown`.
- Refresh on every opening; dismiss/reopen is the retry action. Each result carries
  modal request identity. Serialize inspection work, allowing at most one active
  read/probe and one latest pending request; cancel the obsolete probe and wait
  for cleanup before starting another. An OS stat on a stalled filesystem may
  not be interruptible, so dismissal/quit must not wait for that read to return.

## Acceptance criteria

- [ ] Browser `i` opens information for the exact selected media path in
  Directory and Recursive modes, including filtered results and duplicate names.
- [ ] Filter editing treats `i` as text; help, Queue focus, directories, empty
  results, and pending directory scans cannot trigger inspection.
- [ ] Filesystem fields are accurate, including exact size, zero-byte files,
  timezone, and the selected path of a media-file symlink.
- [ ] Duration, container, bitrate, and all video/audio stream details match probe
  output; missing/invalid values are explicit and cover art is distinguished.
- [ ] Missing ffprobe, corrupt media, timeout, malformed/oversized output, and
  command failures preserve file details and show an actionable warning.
- [ ] Inspection uses the selected file independently of playback, starts no
  playback window, and does not access network resources. Probe cancellation,
  child cleanup, and bounded output/work are covered by automated checks.
- [ ] Loading transitions to information or an error. Deleted files, broken
  links, metadata failures, and changed file types do not crash the application
  or overwrite browser/playback errors.
- [ ] Delayed results after dismissal or reopening cannot display obsolete
  details. Quit and dismissal remain responsive while metadata is pending.
- [ ] Dismissal preserves browsing context, selection, filter, focus, queue,
  and playback; modal keys cannot accidentally play/enqueue or navigate files.
- [ ] Playback events and natural queue advancement continue while inspecting.
- [ ] Wide/narrow layouts, scrolling, resizing, long Unicode/control-character
  paths, and monochrome output remain readable and fit the available frame.
- [ ] In-app help and README document the shortcut, fields, dismissal, and
  modal key behavior.
- [ ] Automated checks pass and representative terminal captures plus user
  interactive acceptance are recorded before status becomes `verified`.

## Questions and dependencies

The user resolved presentation and information scope: use a dismissible overlay
and include media details. No additional user preference blocks planning. The
specific fields, 10-second limit, and graceful fallback above are proposed defaults
for approval with the complete plan.

`ffprobe` is the proposed runtime dependency for media details. Its machine-readable
format/stream inspection avoids implementing media parsers in Go and separates
inspection from playback. Installation must be documented; it must not become a
mandatory application startup check. No new Go module is required.

Design references checked on 2026-10-18:

- [FFmpeg ffprobe documentation](https://ffmpeg.org/ffprobe.html): JSON output,
  `-show_entries`, format/stream sections, and attached-picture distinctions.
- [Debian ffmpeg package file list](https://packages.debian.org/trixie/amd64/ffmpeg/filelist):
  provides `/usr/bin/ffprobe`.

## Implementation plan

1. Record explicit implementation approval for this updated plan under the
   [feature workflow](../FEATURE-WORKFLOW.md).
2. Add `internal/mediainfo` with normalized filesystem/container/stream results,
   optional values, and distinct file/probe failures. Implement bounded,
   cancellable ffprobe execution and JSON parsing behind a small inspection API.
   Keep formatting in the TUI. Inject the runner/stat operation for deterministic
   failure and cancellation tests; add saved JSON fixtures for media variants.
3. Extend `internal/tui/model.go` with modal identity/path, loading/error/result,
   scroll state, and an inspector seam. Run reads as commands, serialize
   pending work, cancel obsolete probes, reject obsolete results, and apply modal
   keys after global quit handling but before browser/playback shortcuts. Continue
   processing asynchronous playback and directory messages.
4. Add rendering in `internal/tui/view.go` or a focused companion file. Reuse
   sanitization, width handling, and help scrolling patterns. Adapt the modal
   to narrow/short terminals. Update README with ffprobe setup and fallback
   behavior, and update in-app help. Add model and rendering tests for the
   acceptance cases.
5. Run automated checks, capture representative views, and supply the user
   repeatable interactive checks. Record results and remaining limitations;
   publish the feature PR after required verification is complete.

## Progress and decisions

- 2026-10-18: User requested planning for MVP-3, selecting the planned feature
  lifecycle. Read its seed request, workflow, browser implementation, TUI model,
  rendering, existing test cases, README, and completed MVP-2 plan.
- 2026-10-18: User approved this plan for implementation, including the
  dismissible overlay, requested media details, bounded probing, cancellation,
  fallback behavior, and verification scope.
- 2026-10-18: Started implementation on `feature/mvp-3-selected-file-information`.
- 2026-10-18: Added `internal/mediainfo` for regular-file stat results, bounded
  cancellable ffprobe execution, local protocol restriction, normalized stream
  metadata, and graceful probe warnings. Added a scrollable `i` modal that
  preserves browser/playback state, ignores stale results, and serializes
  inspection work. Updated README and in-app help. Added parser, filesystem,
  modal state, stale-result, and terminal-frame tests.
- 2026-10-18: `go test ./...`, `go vet ./...`, `go test -race ./...`,
  `go build -o build/mvplayer ./cmd/mvplayer`, and `git diff --check` pass when
  Go uses writable caches under `/tmp`. The workspace default Go cache is
  read-only here. An initial race run with that cache also reported
  `runtime/race: package testmain: cannot find package`; a fresh cache at
  `/tmp/mvplayer-mvp3-race-cache` passed. No installed-media fixture or
  interactive user acceptance has been performed; those checks remain pending.
- 2026-10-18: User verification found that the modal lacked the browser/queue
  backdrop, its close/scroll guidance was detached from the border, and its
  height caused avoidable scrolling. The down-scroll offset also kept growing
  past the last page. Updated the modal to overlay the existing view, use more
  terminal height, put controls in the bottom border, and clamp line/page scroll
  to the current content. Added a regression test that fails with the previous
  unbounded offset and passes with the clamp, plus a wide-frame backdrop check.
  Follow-up user acceptance is pending. The full test, vet, race, build, and
  whitespace checks passed again after the updates using writable caches under
  `/tmp`.
- 2026-10-18: User retested and confirmed the operation looks good, the
  `testdata/` samples fit within the overlay, and the scroll buffering issue is
  no longer reproducible. Rendered wide and narrow terminal text captures are
  recorded in `captures/MVP-3.txt`. This resolves the reported UI findings and
  completes the requested retest. Installed `ffprobe` processed each repository
  media sample with the configured local-protocol whitelist.
- 2026-10-18: Capture review exposed a binary-size unit label off by one step
  and a truncated narrow border hint. Corrected both and added binary-unit
  regression coverage. Full automated checks passed again.
- 2026-10-18: Corrected the seed's copied MVP-1 heading and expanded its intent
  into this proposal. Planning edits remain local on `develop`; no implementation
  approval, branch, commit, push, or PR is recorded.
- 2026-10-18: Recommended a dismissible modal and basic filesystem information;
  requested the user's presentation and metadata preferences.
- 2026-10-18: User selected the dismissible overlay and requested duration,
  codecs, resolution, and other media information. Expanded scope to a separate
  ffprobe inspector, per-stream video/audio fields, bounded asynchronous probing,
  cancellation, and graceful fallback to file details. Checked FFmpeg and Debian
  primary documentation.

## Verification instructions

Agent performs these checks after implementation:

1. `go test ./...`, `go vet ./...`, and `go test -race ./...`.
2. `go build -o build/mvplayer ./cmd/mvplayer` and `git diff --check`.
3. Use temporary fixtures with known bytes/timestamps, an empty media file,
   duplicate basenames under different folders, a valid file symlink, a broken
   link, and a file removed or replaced by a directory before reading. Verify
   selected paths and expected values/errors. Simulate access errors through
   the inspector seam.
4. Delay metadata responses deterministically. Open, dismiss, and reopen for a
   different file; deliver obsolete results and confirm they are ignored.
   Verify one inspection at a time and bounded pending requests. Deliver
   playback/EOF and resize events while the modal is open; verify processing and
   preservation of browsing context.
5. Render loading, success, and error at 120x30, 100x24, 80x24, 40x12, 24x10,
   and below-minimum dimensions, including long Unicode paths, control
   characters, scrolling, and `NO_COLOR`. Check visible widths and line counts.
   Capture representative wide/narrow terminal views for the acceptance record.
6. Test parsing with saved JSON for video/audio, audio-only with cover art,
   multiple tracks, absent fields, `N/A`, invalid rationals/numbers, unknown keys,
   and malformed output. Use a fake executable/runner to exercise missing binary,
   nonzero exit, timeout, cancellation, output caps, and process reaping without
   depending on installed codecs or media devices.
7. With ffprobe installed, inspect small known audio/video fixtures and compare
   displayed fields with its JSON output. Include audio with cover art and
   multiple streams. Document fixture sources or generation commands. Verify
   probing another file while playback runs does not affect mpv. Check network
   protocol restriction using an input that refers to a network resource.

User performs interactive acceptance with the supplied build:

1. Select a file, press `i`, inspect the agreed fields, scroll long paths, and
   dismiss with each documented key. Confirm the same row, filter, mode, and
   browser position remain. Repeat with filtered recursive results and symlinks.
2. Open information during playback, resize, and allow a queued item to finish.
   Confirm playback and queue progression continue and the selected file's
   information stays consistent until dismissal/reopening.
3. Check a narrow terminal and `NO_COLOR` for readable fields, title, and hints.
   Confirm errors for a removed file are understandable and dismissible.
4. Inspect known audio/video files and a multitrack or cover-art example. Confirm
   the displayed metadata is useful and accurate. With ffprobe absent from the
   application's PATH, verify filesystem details and installation guidance remain
   accessible and normal browsing/playback still work.

## Verification results

Automated test, vet, race, build, and whitespace checks passed after the
follow-up presentation fixes. The user confirmed the modal layout and scrolling
behavior against the repository's sample media. Installed ffprobe accepted all
repository media samples using the configured local-only protocol list.
Representative rendered captures are recorded in `captures/MVP-3.txt`;
automated frame checks cover 120x30, 100x24, 80x24, 40x12, and 24x10.

## Merge and archive

Pending. Record PR, merge commit, and merge date after confirmed merge.
