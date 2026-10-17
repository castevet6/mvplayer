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

Git history is unavailable, so no existing commit convention can be confirmed. Use concise, imperative subjects such as `Add playback state model`. Keep commits focused. In pull requests, explain behavior changes, link relevant issues, and report validation performed. Include terminal captures for visible TUI changes and note Debian-specific dependencies or setup requirements.

## Pull requests and review

Codex orchestrates review and fixes. Claude reviews the PR head through
`scripts/review-pr.sh`, following `REVIEW.md`. Opening or updating a PR authorizes
running the review script, posting review responses, and committing and pushing
fixes within the authorized scope. Existing feature approval gates still apply.

Before starting work that needs GitHub CLI access, confirm that network-capable
execution is available. If it is not already available, request escalated
execution for the required `gh` commands before proceeding. Treat an API
connection failure inside the sandbox as a possible network restriction; check
GitHub CLI authentication in network-capable execution before concluding that
credentials are invalid.

After opening or updating a pull request:

1. Run `./scripts/review-pr.sh <pr-number>` from the repository root and wait for
   it to finish; it can take a few minutes. The script uses `bot-token.sh` for
   the reviewer identity. Keep tokens and private keys out of logs and comments.
2. Read the bot's inline findings, review summaries, and PR conversation. Replace
   `{owner}/{repo}` and `<pr-number>` with the actual values:

   ```bash
   gh api --paginate repos/{owner}/{repo}/pulls/<pr-number>/comments --jq '.[] | select(.body | contains("claude-review")) | {id, path, line, body}'
   gh api --paginate repos/{owner}/{repo}/pulls/<pr-number>/reviews --jq '.[] | select(.body | contains("claude-review")) | {id, commit_id, body}'
   gh pr view <pr-number> --comments
   ```

   Read open review threads from other reviewers too, including earlier rounds.
   The script may post a fallback conversation comment instead of inline findings.
3. Fix every valid **blocking** finding. Fix **suggestion** findings when clearly
   right; address **nit** findings only when trivial. Address actionable open
   review comments within scope. If a blocking finding is disputed, reply in its
   thread with the reason and evidence. Report findings that require scope
   approval or cannot be fixed, and leave them open.
4. Run affected checks and repository-required checks, commit and push fixes,
   and reply to addressed threads with the fix and validation. Run the review
   again after fixes, up to **two review rounds total per PR**, counting earlier
   sessions and fallback reviews. Read and address findings from the second round
   too; push those fixes without starting a third round.
5. Report the PR URL, rounds used, validation, and any remaining findings or
   changes made after the last review. If the script fails, report the failure;
   a failed review is not a clean review.

Never merge a pull request; a human performs merges. Never approve your own PR
or dismiss the bot's review.
