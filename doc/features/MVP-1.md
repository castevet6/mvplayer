# MVP-1: Clear pane focus and selection

- Status: verified
- Implementation approved: yes
- Approved by/date: User / 2026-10-17
- Branch: feature/mvp-1-clear-pane-focus-and-selection
- PR: https://github.com/castevet6/mvplayer/pull/1
- Verification: passed

## Feature status

- **Implementation progress:** Updated the focus palette after the user's visual
  review found the selected heading difficult to read. Active headings now use
  bright white on blue; active rows use bold underline with the terminal's
  default foreground. README and help describe the revised cues.
- **Validation:** Full tests and vet pass after the palette revision. Visual
  review of the revised palette has passed on dark and light backgrounds,
  including wide and narrow layouts and `NO_COLOR` output.
- **Blockers:** None identified during planning.

Mark the feature complete only after the acceptance criteria below have been met.

## Approval and decisions

On 2026-10-17, the user explicitly approved the current plan by requesting
“Mark MVP-1 ready for implementation.” Approval covers the visual and interaction
specification, responsive layout, rendering and color behavior, acceptance
criteria, and documentation changes described below.

On 2026-10-17, the user requested implementation. Work began on
`feature/mvp-1-clear-pane-focus-and-selection`, based on fetched `origin/develop`
with the local feature-workflow commit carried onto the branch.

After reviewing the first implementation, the user reported weak contrast in
the focused-pane colors and requested a palette change. The revised scheme uses
bright white on standard blue for the active heading and bold underline with
the terminal's default foreground for the active row, so both adapt to dark and
light terminal backgrounds. This supersedes the initial cyan palette.

## Type

User story

## Problem and intended outcome

Tab switches focus between Browser and Queue, but the current `>` heading marker
does not clearly identify the active pane. Both lists also retain a `>` cursor,
making keyboard focus easy to confuse with remembered row selection. The Queue
uses a separate `*` marker for playback.

Make keyboard focus immediately recognizable in wide and narrow terminals.
Coordinate heading and row styling, preserve a quiet indication of each pane's
remembered selection, and distinguish selection from queue playback. Require
Queue focus before `d` can remove an item.

Use high-contrast standard ANSI colors, with text cues that remain
understandable without color. Useful precedents are
[Lazygit's separate active-pane and retained-selection styles](https://github.com/jesseduffield/lazygit/blob/master/docs/Config.md#default)
and [Yazi's separate active-tab and list-indicator styles](https://yazi-rs.github.io/docs/configuration/theme/#tabs).

## Visual and interaction specification

- **Active heading:** Bracket the pane name, use bold bright white text on a
  standard blue
  background, and retain its position/count. In wide layouts, extend the
  background across that pane's heading width.
- **Inactive heading:** Use plain terminal colors without brackets. Keep its
  position/count visible.
- **Active selected row:** Retain `>` and apply bold, underlined text using the
  terminal's default foreground color to the marker and filename. Do not add a
  background band.
- **Inactive selection:** Retain a plain `>` on the remembered row, without
  dimming the filename.
- **Playback marker:** Preserve `*` independently of focus and selection,
  including the existing combined `>*` marker. Describe these meanings in help.
- **Empty panes:** Heading treatment still identifies focus; do not invent a
  selected row.
- **Filtering:** Browser remains focused, with `Filter (editing):` in its
  context line. Tab finishes editing and focuses Queue, preserving the existing
  behavior.
- **Deletion:** `d` acts only with Queue focus. Preserve existing restrictions
  on removing current or completed queue entries. Keep `[` / `]` and playback
  controls global.

## Responsive layout

- Preserve the 100-column breakpoint.
- At 100 columns and above, retain two pane columns and the existing separator.
- Below 100 columns, show both pane names in one header, bracket and highlight
  the active name, and include `Tab switch`. Render only the active pane's list.
- Use the next line for the active position/count and directory or filter
  context. Prioritize the count, then truncate context to available width.
- Move queue shortcut guidance into the existing footer, removing the extra
  narrow Queue instruction row. Both narrow panes then use the same list
  capacity; update scrolling calculations accordingly.
- Preserve the minimum terminal size of 24 × 10 and existing resize guidance.

Illustrative text structure (color and bold are omitted here):

```text
Wide:
[Browser]  3/18                  |  Queue  2/5
> concert.mkv                    | > track.flac
                                 | * interview.mp4

Narrow:
[Browser] Queue     Tab switch
3/18  /home/user/videos
> concert.mkv
```

## Rendering implementation

Use the existing `x/ansi` styling facilities; add no dependency or exported API.
Keep color capability and environment state inside the TUI model.

Sanitize filenames, paths, queries, and error strings **before** adding trusted
styling. The current renderer sanitizes completed lines and would damage ANSI
styling sequences. Make layout helpers ANSI-aware and stop sanitizing completed
styled lines. Ensure heading and row styles reset before separators, adjacent
panes, and subsequent lines.

## Color behavior

- Use standard ANSI blue and bright white rather than requiring true color.
- Respect any nonempty `NO_COLOR` value, including `0` and `false`, following
  the [NO_COLOR convention](https://no-color.org/).
- With `NO_COLOR`, use brackets and bold only; emit no foreground/background
  colors or reverse video.
- For terminals without styling support, use plain text.
- Start conservatively until terminal capability and environment messages
  arrive. Presentation changes must preserve focus, cursors, and scroll
  positions.
- Add no theme configuration, mouse controls, or new navigation shortcuts.

## Acceptance criteria and validation

Add focused Go tests covering:

- Tab switching in both layouts, independent retained cursors, and focus on
  empty panes.
- Separate selection and playback markers, including `>*`.
- Queue-only deletion and preserved global queue selection shortcuts.
- Filter editing and Tab exit behavior.
- Layouts at 24, 99, and 100 columns; resizing; scrolling; long Unicode
  filenames.
- Color, bold/underline, bold-only, and plain output; nonempty `NO_COLOR` variants.
- ANSI display widths, style reset boundaries, and control characters in
  filenames, paths, queries, and errors.

Run `go test ./...` and `go vet ./...` during implementation. Capture
representative wide and narrow terminal views on dark and light backgrounds,
including monochrome output.

The feature is complete when the agreed focus and selection treatments work in
both layouts and all output modes, deletion obeys Queue focus, rendering remains
within terminal dimensions, and the automated checks and visual review pass.
Record the validation evidence and any remaining limitations in the feature
status section.

## Documentation and scope boundaries

Update README and in-app help for the visual cues and deletion rule. Keep this
feature document current as implementation proceeds.

Playback architecture and queue sequencing remain outside this feature.

## Verification results

Automated verification on 2026-10-17, rerun after the palette revision:

- `env GOCACHE=/tmp/mvplayer-gocache go test ./...` — passed.
- `env GOCACHE=/tmp/mvplayer-gocache go vet ./...` — passed.
- `gofmt -w internal/tui/model.go internal/tui/view.go internal/tui/model_test.go`
  — completed.

Manual verification on 2026-10-17, performed by the user after the palette
revision:

- Reviewed wide and narrow layouts on dark and light terminal backgrounds.
- Tested `NO_COLOR`; brackets and bold/underline text cues remained visible
  without terminal colors.
- Confirmed the revised palette is legible and accepted.
