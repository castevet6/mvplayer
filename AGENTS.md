# Repository Guidelines

## Feature Workflow

For MVP feature planning, approval, implementation, verification, PR creation,
or post-merge archiving, read `doc/FEATURE-WORKFLOW.md` and use the repository
skill at `.agents/skills/mvplayer-feature/SKILL.md`.

For GitHub PR creation, updates, or review-comment follow-up, use
`.agents/skills/github-pr/SKILL.md` and follow the pull request review policy below.

Feature plans are tracked in `doc/features/MVP-<number>.md`. Implementation
requires explicit user approval recorded at the beginning of the plan. Missing,
ambiguous, or negative approval blocks implementation: explain the gate and wait
for explicit approval or an explicit exception. A request to implement alone
does not override this gate. Record exceptions in the plan.

Implement approved features on `feature/mvp-<number>-<hyphenated-description>`
branches based on `develop`; target feature PRs at `develop`. Approval to mark a
plan ready does not itself request implementation. Follow the workflow for
scope changes, verification, and archiving after merge.

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
