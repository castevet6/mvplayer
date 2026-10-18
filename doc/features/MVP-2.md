# MVP-2: Recursive browsing mode

- Status: implemented
- Implementation approved: yes
- Approved by/date: User, 2026-10-18
- Branch: feature/mvp-2-recursive-browsing-mode
- PR: —
- Verification: automated passed; user visual/playback checks pending

## Problem and intended outcome

The browser currently lists only the immediate children of its browsing directory.
Finding media spread across several subdirectories requires repeated navigation.
Add an optional recursive mode that presents recognized media beneath the current
browsing root as one list, makes each file's location clear, and retains folder
navigation without changing playback or the queue.

The original request asks for all files in the current directory and its
subdirectories, visible folder context, and continued folder navigation. The
user approved the interaction below and authorized implementation on 2026-10-18.

## Scope and boundaries

### Approved interaction

- Start in the existing nonrecursive mode. Add `r` to toggle recursive mode while
  Browser has focus and filter editing/help are inactive. While editing a filter,
  `r` remains text input. Queue focus does not toggle the browser mode.
- Keep the current browsing root visible in the browser context and show an
  explicit `Recursive` or `Directory` mode label in wide and narrow layouts.
- In recursive mode, show immediate nonhidden subfolders first as navigable rows,
  then one contiguous list of recognized media from the root and all descendant
  nonhidden folders. Do not add descendant folder rows among the media.
- Sort each group case-insensitively, using relative paths for media and a stable
  case-sensitive tie-breaker. Each media path occurs once. Keep existing sorting
  in nonrecursive mode.
- Display media as root-relative paths, for example `album/live/song.flac`;
  root-level media appear as `song.flac`. Navigation rows retain a trailing `/`.
  Preserve selection/focus styling, sanitize filesystem text before styling,
  and keep rows within the pane width. The help view's selected full path remains
  available when a long relative path is clipped.
- Enter on a folder changes the browsing root to that folder and scans beneath
  it; Backspace moves the root to its parent. Keep recursive mode enabled across
  navigation. Navigation clears the filter as it does today.
- Enter on media plays that exact absolute path; `a` enqueues it. Changing the
  browser root or mode preserves current playback and all queued paths.
- `/` searches the displayed relative path case-insensitively in recursive mode,
  allowing folder-name searches. Filter folder rows by their displayed names.
  Finishing filter editing retains the query. Toggling mode retains the query
  and selected absolute path when it remains visible; otherwise select the first
  visible entry. An empty result has no actionable selection.

### Traversal and failure behavior

- Use the existing recognized audio/video extensions. Omit hidden files and
  hidden directories at every depth. Do not follow directory symlinks, avoiding
  cycles and traversal outside the tree through links; retain existing handling
  of recognized media-file symlinks. This is browsing, not a filesystem sandbox.
- Run traversal outside the Bubble Tea update loop. Cancel obsolete scans on
  navigation, mode changes, and shutdown, and reject stale results using request
  identity, root, and mode. Cancellation must be checked between filesystem
  operations; it cannot interrupt a blocked OS filesystem read.
- Show a scanning indicator and clear obsolete actionable rows while a new root
  or mode is loading. Keyboard navigation, focus changes, playback controls,
  and quit remain responsive. Publish one sorted result when scanning finishes;
  incremental results and filesystem watching are deferred.
- A root read failure shows an actionable browser error. If a descendant cannot
  be read or disappears, retain other results and show a partial-results warning
  identifying a failed path and the number of skipped subtrees. Do not present
  a partial scan as complete success or mix it with playback errors.

No persistent library/index, saved mode, CLI options, bulk enqueue, recursive
autoplay, new dependencies, playback changes, queue sequencing changes, or
independent folder pane are included. A flat listing alone does not start or
advance playback. No arbitrary depth or file-count cap is proposed.

## Acceptance criteria

- [ ] Default startup and nonrecursive browsing preserve established behavior.
- [ ] Browser-only `r` toggles mode; filter input, help, and Queue focus obey the
  proposed key rules. Root and mode remain recognizable in both layouts.
- [ ] Recursive results contain every eligible media path once, with immediate
  folder navigation rows above a deterministically sorted media list.
- [ ] Duplicate basenames in different folders are distinguishable by relative
  path, and both play/enqueue the correct absolute file.
- [ ] Enter/Backspace change root while retaining mode; neither action interrupts
  playback nor changes pending queue items.
- [ ] Filtering matches relative paths; mode changes retain query and selection
  where possible, and empty results cannot play or enqueue an obsolete item.
- [ ] Hidden subtrees and directory symlinks are not traversed. Empty trees,
  unreadable/disappearing descendants, and root failures have the specified UI.
- [ ] Delayed results from obsolete scans cannot replace the current listing or
  warning. Obsolete scans stop cooperatively and quit remains responsive.
- [ ] Wide/narrow views, resizing, scrolling, long Unicode paths, control
  characters in filesystem names, and monochrome output remain usable and bounded.
- [ ] Required automated checks and user visual/playback checks pass, with
  representative terminal captures recorded before verification is complete.

## Questions and dependencies

1. The user approved immediate folders above recursive media, with Enter changing
   the recursive root; no separate navigation view is included.
2. The user approved `r`, full relative-path rows, and filtering folder rows
   alongside media.
3. Very large or network-backed trees may finish slowly. The approved design keeps the
   UI responsive and permits cancellation; incremental display can be reconsidered
   if the user's expected library size requires it.

No additional dependency is expected. The recorded interaction approval covers
implementation under [the workflow](../FEATURE-WORKFLOW.md).

## Implementation plan

1. Resolve the navigation questions and record the agreed interaction and explicit
   implementation approval before starting code changes.
2. Extend `internal/browser` with a cancellable recursive listing operation and
   enough result information to distinguish fatal errors from skipped-subtree
   warnings. Keep file identity separate from display paths and navigation rows.
   Cover traversal, sorting, filtering, hidden paths, symlinks, cancellation,
   and partial failures with temporary-directory tests and a controllable read
   seam where real permission errors would be unreliable.
3. Extend `internal/tui/model.go` with mode, scan lifecycle, cancellation, and
   stale-result protection. Adapt the existing injected lister seam; test toggle,
   navigation, selection retention, filtering, warnings, and exact file actions
   through model messages and a fake player.
4. Update `internal/tui/view.go` for mode/root context, relative paths, scan state,
   and warnings in both layouts. Add focused rendering tests and update README,
   in-app help, and the project design's nonrecursive default description.
5. Run automated checks, capture representative terminal views, and request the
   user's visual and real-media verification. Record actual evidence before
   publishing under the feature and GitHub PR workflows.

## Progress and decisions

- 2026-10-18: User requested planning for MVP-2. Inspected the browser listing,
  TUI loading/navigation/rendering, existing tests, and feature workflow.
- The seed note was untracked at `doc/features/done/MVP-2.md`, had an MVP-1
  heading, and contained no approval, implementation, or merge evidence. Moved
  it into the active feature directory and corrected its identifier. Its intent
  is preserved above; MVP-2 is not treated as an archived completed feature.
- The previous lister read one directory asynchronously; stale results were
  guarded by request number and path. This implementation extends the scan
  message identity to include mode, adds cooperative recursive cancellation,
  relative-path filtering, and partial-result warnings.
- 2026-10-18: User approved the proposed interaction and authorized
  implementation. Status changed to `implementing` on
  `feature/mvp-2-recursive-browsing-mode`.
- 2026-10-18: Added cancellable recursive traversal, path-aware filtering and
  rendering, mode/root loading state, stale scan rejection, and browser/queue
  documentation. Browser and TUI coverage includes filtering, cancellation,
  partial failures, mode retention, stale warnings, and exact file actions.
- 2026-10-18: Implementation and automated verification completed. Status is
  `implemented`; user visual and playback acceptance remains pending.

## Verification instructions

Agent performs automated checks after implementation:

1. `go test ./...` and `go vet ./...`.
2. `go test -race ./...` for scan cancellation and asynchronous result handling.
3. `go build -o build/mvplayer ./cmd/mvplayer` and `git diff --check`.
4. Use a temporary tree containing root media, `album/live/song.flac`, a second
   `song.flac` in another folder, unsupported/hidden files, an empty folder,
   hidden subtrees, and a directory-symlink cycle. Assert exact result paths and
   order, filter matches, and correct action paths. Simulate read failures and
   delayed scans deterministically rather than depending on filesystem timing.
5. Exercise mode/root changes while a scan is pending, then deliver old results;
   verify stale rows/warnings are rejected and cancellation is signaled. Verify
   selection/query behavior and preserved playback/queue state.

User performs interactive acceptance after the agent supplies a build and fixture
instructions; agent captures representative terminal output where possible:

1. Browse a nested media tree at wide (at least 100 columns) and narrow widths.
   Toggle mode, enter a folder, move to its parent, filter by folder and filename,
   resize, and inspect long selected paths through help. Confirm location, mode,
   file identity, focus, and scrolling remain clear, including with `NO_COLOR`.
2. Play/enqueue two files with matching basenames from different folders. Confirm
   the intended files play and navigation/toggling preserve playback and queue.
3. Scan a large tree, switch root/mode, and quit while scanning. Confirm the UI
   remains responsive; check partial-result warnings with a suitable failing
   subtree where the environment permits. Deterministic tests cover read failures
   when the user's environment cannot reproduce them.

## Verification results

Automated verification completed on 2026-10-18:

- `env GOCACHE=/tmp/mvplayer-gocache go test ./...` — passed.
- `env GOCACHE=/tmp/mvplayer-gocache go vet ./...` — passed.
- `env GOCACHE=/tmp/mvplayer-gocache go test -race ./...` — passed.
- `env GOCACHE=/tmp/mvplayer-gocache go build -o build/mvplayer ./cmd/mvplayer` — passed.
- `git diff --check` — passed.

An 80×24 PTY run with root media plus `album/live/song.flac` and
`other/song.flac` showed the default `Directory` label, then the `Recursive`
label and both relative paths after pressing `r`. The root and mode remained
visible, and the existing folder selection remained selected. A repeatable
terminal excerpt:

```text
[Browser]  Queue
1/3  <fixture-root>  Directory
> album/
  other/
  song.flac

[Browser]  Queue
1/5  <fixture-root>  Recursive
> album/
  other/
  album/live/song.flac
  other/song.flac
  song.flac
```

User visual and real-media acceptance remains pending. Follow the checks above
at wide and narrow widths, verify matching-basename playback/enqueue, and test
responsiveness while scanning before marking the feature verified.

## Merge and archive

Pending implementation, verification, PR, and confirmed merge into `develop`.
