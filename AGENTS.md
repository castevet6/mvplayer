# Repository Guidelines

## Change workflow

Choose the delivery path from scope and risk, and state the choice briefly.
Proceed without a classification approval when the choice is clear. Follow
explicit user instructions about publication and review.

| Path | Use when | Delivery |
| --- | --- | --- |
| Direct maintenance | Small, bounded, low-risk work, such as documentation clarifications, routine skill edits, or minor fixes preserving established behavior | Validate, commit, and push directly to `develop`. |
| Change with review | Substantial fixes, refactoring, dependencies, CI, or changes affecting playback, persistence, permissions, or other sensitive behavior | Branch from `develop`, validate, and open a PR into `develop`; follow `.agents/skills/github-pr/SKILL.md`. No MVP document is required. |
| Planned feature | The user explicitly starts or resumes the MVP feature lifecycle | Follow `doc/FEATURE-WORKFLOW.md`, with an MVP document, recorded approval, feature branch, PR, and post-merge archiving. |

Assess behavior and risk rather than file type or line count. Recommend formal
feature planning when useful, but let the user choose it; ordinary requests do
not create `MVP-*.md` documents automatically.

### Direct maintenance

An implementation request on this path authorizes affected checks, a focused
commit, and a normal push to `develop`, unless the user limits publication.
Planning or proposal requests alone do not authorize commits or pushes.

Inspect the working tree and remote history before updating local `develop`.
Preserve unrelated work and unique local commits; use a separate worktree when
needed. Continue existing in-flight work on its established path unless the
user changes it; prepare unrelated maintenance on `develop`, rather than adding
it to an active or merged PR branch.

Run checks appropriate to the change, inspect the final diff, commit, and push
normally. Report validation, the commit, and whether it reached remote
`develop`. If protection rejects the push, preserve the commit and report the
restriction; create a PR only when requested. Direct maintenance requires no
Claude PR review or feature archive.

### Planned features

Start the lifecycle explicitly with the `feature` skill at
`.agents/skills/feature/SKILL.md` (invoke as `$feature`, or select it in the
client's skill interface). Once started, the selected feature remains active
through follow-up requests and merge notification; repeated invocation is
unnecessary. For an existing feature context or an explicit request to resume
`MVP-<number>`, read `doc/FEATURE-WORKFLOW.md` and its feature document.

Feature implementation requires explicit user approval recorded at the
beginning of `doc/features/MVP-<number>.md`. Missing, ambiguous, or negative
approval blocks implementation until explicit approval or an explicit exception
is recorded. An implementation request alone does not override this gate.
Approval to mark a plan ready does not itself request implementation.

## Project Scope & Structure

MVPLAYER (MyVideoPlayer) is a Linux CLI video player with a terminal UI, primarily targeting Debian. Use Go, favor the standard library, and use Bubble Tea for the UI, as specified in `doc/PROJECT-PLAN.md`.

Project planning documents live in `doc/`. Place the executable entry point in `cmd/mvplayer/`, private application packages in `internal/`, and Go tests beside the code they exercise. Store small test fixtures in package-local `testdata/` directories.

## Build, Test, and Development Commands

Run these from the repository root once a Go module and the proposed entry point exist:

- `go run ./cmd/mvplayer` — launch the application locally.
- `go build -o build/mvplayer ./cmd/mvplayer` — compile the player into the ignored build output directory.
- `go test ./...` — run all Go tests.
- `go vet ./...` — check for common Go mistakes.
- `gofmt -w <changed-files>` — format modified Go files.

The Makefile also builds to `build/mvplayer`. Keep generated executables and other local build outputs under `build/`, which is ignored by Git.

## Coding Style & Naming Conventions

Use `gofmt` formatting, including tabs for Go indentation. Use lowercase package names, exported `PascalCase` identifiers, and unexported `camelCase` identifiers. Keep playback logic separate from Bubble Tea state and rendering. Justify dependencies beyond the standard library and the planned UI framework.

## Testing Guidelines

Use Go’s `testing` package with `*_test.go` files and `TestXxx` functions. Prefer table-driven tests for input handling and state transitions. Keep automated tests independent of a real terminal or media device where practical. No coverage threshold is established.

## Commit & Pull Request Guidelines

Use concise, imperative commit subjects and keep commits focused. In pull
requests, explain behavior changes, link relevant issues, and report validation
performed. Include terminal captures for visible TUI changes and note
Debian-specific dependencies or setup requirements.

## Pull requests and review

For PR creation, review, or follow-up, follow `.agents/skills/github-pr/SKILL.md`.
It defines one-per-chat GitHub execution approval, publication authorization,
review rounds, findings, and the handoff to the human merger. Feature approval
gates in `doc/FEATURE-WORKFLOW.md` remain authoritative.
