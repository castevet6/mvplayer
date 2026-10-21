# MVP-4: Hide folders from recursive view

- Status: verified
- Implementation approved: yes
- Approved by/date: User, 2026-10-21
- Branch: feature/mvp-4-hide-folders-in-recursive-view
- PR: https://github.com/castevet6/mvplayer/pull/5
- Verification: automated and user acceptance passed 2026-10-21

## Problem and intended outcome

Recursive browsing shows immediate subfolders before media from the browsing
root and its descendants. When media are scattered across many subfolders, users
must scroll through several pages of folder rows before reaching files.

Allow users to hide those navigation rows and browse the recursive media list
directly, then show folders again when they want to enter one. Preserve the
existing recursive traversal and relative-path context.

## Scope and boundaries

### Proposed interaction

- Add `v` to toggle folder visibility when Browser has focus and recursive mode
  is active. Ignore it in Directory mode and with Queue focus. In filter editing,
  `v` remains text input; help and the file-information modal retain their
  existing key handling.
- Start each application session with folders shown. Remember the recursive
  visibility choice across root navigation, focus changes, filtering, and
  switching Directory/Recursive mode within that session. Directory mode always
  shows folders normally. Restarting resets the choice to shown.
- With folders hidden, omit all directory navigation rows, including immediate
  directory symlinks. Keep every eligible media entry, its root-relative display
  path, absolute action path, and existing sorted order. Hiding a folder row
  does not exclude its subtree's media from the scan.
- Toggling visibility updates the current results immediately without starting
  or cancelling a filesystem scan. If a scan is already running, remember the
  choice and apply it when the accepted result arrives. Keep scanning state,
  root errors, partial-result warnings, and stale-result protection intact.
- Retain the filter query. Continue matching media by root-relative path, so
  searching a folder name still finds media inside it. Showing folders again
  restores only folder rows matching the current query.
- Retain the selected absolute path if it is still visible. When hiding the
  selected folder, select the first visible media entry; if there is none, leave
  no actionable selection. Keep cursor, scrolling, and visible row counts
  consistent with the resulting list. Showing folders retains the selected
  media rather than jumping back to the folder prefix.
- Backspace still opens the parent root. To enter a child folder, show folders
  with `v` and use Enter. Keep the visibility choice after navigation; navigation
  clears the query as it does today.
- Show `Folders shown` or `Folders hidden` with recursive browser context in
  wide and narrow layouts. Keep the visibility state readable even when a long
  root path needs truncation. Add the shortcut and its scope to in-app help,
  the Browser hints where space permits, and README.
- If no media match while folders are hidden, show a media-specific empty
  message with a `v` hint to show folders. Keep `Scanning...` during loading and
  preserve any error or partial-results notice.
- Enter, `a`, and `i` continue acting on the selected visible media's exact
  absolute path. A visibility change preserves playback and queue state.

This is a session-only display option. No persistent configuration, CLI flag,
new dependency, traversal-policy change, subtree exclusion, sorting change,
bulk enqueue, or playback change is included. Existing hidden-file omission and
directory-symlink traversal rules remain in force.

## Acceptance criteria

- [x] Startup preserves Directory mode and the existing folder listing; the
  first recursive view shows folders under the proposed default.
- [x] Browser-only `v` toggles folder rows in recursive mode; Directory mode,
  Queue focus, filter editing, help, and file information follow the key rules.
- [x] Hiding folders leaves the same ordered media paths, including nested
  files and duplicate basenames with distinguishable relative paths. Showing
  folders restores the matching immediate folder and directory-symlink rows.
- [x] The query survives toggling; a folder-name query still matches descendant
  media while folder rows are hidden.
- [x] A selected media path survives both toggle directions and remains in the
  viewport. Hiding a selected folder selects the first matching media; an empty
  result cannot play, enqueue, or inspect an obsolete entry.
- [x] The choice survives navigation and Directory/Recursive mode changes within
  the session; Directory mode still permits normal folder navigation.
- [x] Toggling does not rescan, alter scan identity, or cancel a pending scan.
  An accepted result uses the latest visibility choice; obsolete results remain
  rejected. Loading, error, and partial-warning states remain accurate.
- [x] Wide/narrow layouts, resizing, scrolling, long and Unicode paths, and
  `NO_COLOR` output show usable state and stay within terminal bounds.
- [x] File actions target the correct absolute paths. Visibility changes leave
  current playback and queued entries intact.
- [x] Required automated checks and user interactive acceptance are recorded
  before marking the feature verified.

## Questions and dependencies

- User approved the proposed `v` toggle with folders shown initially on
  2026-10-21. This approval covers the current scope and acceptance criteria.
- No new runtime or Go dependency is expected. Existing browser entries already
  distinguish directories from media, and the TUI retains unfiltered results
  separately from visible rows.
- Implementation requires explicit approval recorded under
  [the feature workflow](../FEATURE-WORKFLOW.md).

## Implementation plan

1. Resolve the default/shortcut preference and record explicit approval of the
   current scope. On an implementation request, prepare
   `feature/mvp-4-hide-folders-in-recursive-view` from current remote `develop`
   under the feature workflow.
2. Add session-only folder visibility state to `internal/tui/model.go`. Derive
   visible rows from the full listing using the recursive visibility choice and
   existing query filtering. Retain the complete scan result so showing folders
   needs no filesystem read; keep `internal/browser` traversal unchanged.
3. Handle `v` under the existing focus/modal/filter rules. Preserve selection by
   absolute path, define the first-media fallback explicitly, and recalculate
   the viewport. Apply the same visibility rule to incoming scan results and
   mode/root transitions without changing scan identity or cancellation.
4. Update `internal/tui/view.go` for readable folder state, accurate empty-state
   text, help, and contextual hints. Update README's recursive-browsing guidance.
5. Add focused model/render tests for the acceptance cases, using existing
   injected listers, delayed result messages, and fake playback seams. Verify
   both toggle directions, empty results, selection, key contexts, and exact
   file actions without requiring a real terminal or media device.
6. Run the automated checks, capture representative terminal views, and provide
   a build and fixture instructions for user acceptance. Record evidence before
   publication under the feature and GitHub PR workflows.

## Progress and decisions

- 2026-10-21: User requested planning for MVP-4. Expanded the existing untracked
  problem statement on `develop`, preserving its intent. Read the feature
  workflow, template, completed MVP-2 plan, browser traversal, TUI scan/filter/
  selection/key/rendering paths, existing regression tests, and README.
- The existing recursive scan already returns folders and media separately
  identified in one sorted listing. A TUI visibility option can reuse those
  results without changing filesystem traversal or the playback adapter.
- Proposed interaction and default are awaiting user feedback and approval.
-  No implementation changes, branch creation, commits, or publication have
  been performed for this planning request.
- 2026-10-21: User approved the plan and authorized implementation. Status changed
  to `implementing` on `feature/mvp-4-hide-folders-in-recursive-view`.
- 2026-10-21: Added session-only folder visibility state. Browser-focused `v`
  toggles recursive folder rows without rescanning; media paths, filtering,
  selection fallback, playback, and queue state are preserved. Updated rendering,
  help, README guidance, and model regression tests. Status changed to
  `implemented`; interactive layout and user acceptance remain pending.
- 2026-10-21: Fixed the toggle regression found during interactive review. Empty
  queries had returned the original entries slice; filtering hidden folders by
  reusing its backing array overwrote directory rows, so a second `v` could not
  restore them. Filtering now allocates an independent visible slice, and hidden
  recursive media render by basename while shown folders retain relative paths.
  Added regression assertions for both toggle directions and display text.
- 2026-10-21: User confirmed the corrected implementation works interactively
  and approved moving forward with PR publication. Status changed to `verified`.
- 2026-10-21: Opened [PR #5](https://github.com/castevet6/mvplayer/pull/5)
  targeting `develop` after pushing the verified feature branch.

## Verification instructions

Agent performs these checks after implementation:

1. `go test ./...`, `go vet ./...`, and `go test -race ./...`.
2. `go build -o build/mvplayer ./cmd/mvplayer` and `git diff --check`.
3. Use temporary fixtures containing root media, nested media with duplicate
   basenames, many immediate folders, an empty folder, and an immediate directory
   symlink. Assert exact media paths and ordering before/after toggling, query
   matches, folder restoration, and media/folder selection behavior.
4. Exercise `v` with Browser/Queue focus, Directory/Recursive mode, filter
   editing, help, and file information. Verify session retention through root
   and mode changes and the default on a fresh model.
5. Use a counting/injected lister and controlled delayed results to verify a
   visibility toggle performs no read or cancellation, accepted results use the
   current choice, and stale results cannot replace rows or warnings. Cover
   loading, root failure, and partial warning cases deterministically.
6. Verify play/enqueue/inspect targets and preserved queue/playback state with
   model messages and fake adapters. Check rendering bounds, state visibility,
   row counts, empty messages, scrolling, resizing, and monochrome output.

User performs interactive acceptance after the agent supplies a build and
fixture instructions; agent records representative terminal captures:

1. Open a media tree with enough immediate folders to exceed a screen, press
   `r`, then `v`. Confirm files become directly accessible and `v` restores
   folders. Inspect at wide (at least 100 columns) and narrow (for example 60
   columns) widths, with long paths, resizing, and `NO_COLOR`.
2. Select a media item, toggle twice, and confirm selection remains visible.
   Select a folder and hide folders; confirm selection moves to the first media.
   Search by a folder name and confirm its descendant media remain searchable.
3. Show folders, enter a child, hide folders, use Backspace, and switch `r` off
   and on. Confirm normal Directory navigation and remembered recursive choice.
   Check an empty media result and use its hint to restore folder rows.
4. Play and enqueue distinct media, then toggle folder visibility and navigate.
   Confirm the selected files are correct and playback and queued items remain
   intact. Confirm `i` inspects the selected visible media.

## Verification results

Implementation verification on 2026-10-21:

- `GOCACHE=/tmp/mvplayer-gocache go test ./...` — passed.
- `GOCACHE=/tmp/mvplayer-gocache go test -race ./...` — passed.
- `GOCACHE=/tmp/mvplayer-gocache go vet ./...` — passed.
- `GOCACHE=/tmp/mvplayer-gocache go build -o build/mvplayer ./cmd/mvplayer` — passed.
- `git diff --check` — passed.

The folder-toggle regression fix was rechecked with `GOCACHE=/tmp/mvplayer-gocache
go test -race ./...` — passed.

User verification on 2026-10-21: confirmed recursive folder hiding, basename
rendering while hidden, and restoring folders with a second `v` work correctly.

Automated coverage exercises both toggle directions, key-context restrictions,
selection fallback, preserved relative media paths, duplicate basename
disambiguation, folder-name filtering, and rendered visibility state. User
interactive acceptance on 2026-10-21 confirmed the corrected recursive toggle,
basename display, and restoration behavior.

## Merge and archive

Pending. Record the feature PR URL, merge commit, and merge date after confirmed
merge into `develop`.
