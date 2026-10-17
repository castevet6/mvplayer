# Feature workflow

Feature requests, approved plans, and verification evidence are versioned with
the code. Each feature has one document: `doc/features/MVP-<number>.md`.
Use [the template](templates/feature.md) for new requests. Keep the identifier
unique across active and archived features.

## Status and approval

Place the template's metadata block immediately below the title. Use these
status values:

| Status | Required evidence |
| --- | --- |
| `draft` | Initial request; questions may remain. |
| `planned` | Reviewed scope, acceptance criteria, implementation steps, and verification instructions; awaiting approval. |
| `ready` | User explicitly approved the current plan; approval recorded. |
| `implementing` | Work started on the recorded feature branch. |
| `implemented` | Implementation finished; automated results and remaining manual checks recorded. |
| `verified` | All required automated and manual acceptance checks passed. |
| `complete` | Feature PR confirmed merged into `develop`; merge evidence recorded. |

`Implementation approved` is a separate `yes`/`no` field and remains `yes`
through later stages unless approval is revoked or the approved scope changes.
Record who approved, the date, and what scope was approved in the decisions
section. Never infer approval from an existing plan or an implementation request.
An explicit user exception can authorize implementation before the plan is ready;
record its scope and date, set approval to `yes`, and retain unresolved questions.

Material changes to behavior, acceptance criteria, dependencies, or scope require
renewed approval for the changed scope. Set approval to `no`, explain the change,
and pause dependent implementation. Routine internal implementation choices do
not require renewed approval. Preserve completed work and record deviations.

## Plan and approve

1. Read the request and relevant code and repository guidance. Resolve questions
   that affect behavior; record unresolved questions rather than guessing approval.
2. Update the feature document with intended behavior, scope boundaries,
   acceptance criteria, implementation steps, and verification instructions.
   Set `planned`; make no implementation changes during a planning-only request.
3. When the user explicitly approves, record approval and set `ready`. A request
   to mark ready finishes here unless implementation was also requested.

Planning edits stay on the current appropriate documentation branch. The feature
branch must include the approved plan; preserve uncommitted work when preparing
the implementation checkout.

## Implement

1. Check approval, local changes, and existing branches. Fetch the remote and
   create the feature branch from current remote `develop`, or resume its existing
   branch after inspecting its state. Use a separate worktree when needed to
   protect unrelated work. Carry the approved plan into the feature branch.
2. Name the branch `feature/mvp-<number>-<hyphenated-description>`, for example
   `feature/mvp-1-clear-pane-focus-and-selection`. Set `implementing` and record it.
3. Implement the approved scope. Keep meaningful progress, decisions, remaining
   work, and blockers current in the plan. If sub-agents are authorized and used,
   coordinate document edits through one owner to avoid conflicting updates.
4. Run the required checks and update documentation. Set `implemented` when code
   work is finished. Record exact check commands, outcomes, limitations, and
   repeatable manual verification steps with expected results.

Commit document updates at meaningful milestones alongside related code. Keep
transient scratch notes out of the permanent feature record.

## Verify and open the PR

Run `go test ./...` and `go vet ./...`, plus feature-specific checks. For visible
TUI changes, record representative terminal captures. Identify who performs each
manual check; user-only checks remain pending until the user reports results.
Record dates and evidence. Failed or unperformed required checks prevent
`verified`. Changes after verification require repeating affected checks.

A request to implement a feature under this workflow authorizes local commits,
pushing its feature branch, and opening a PR into `develop` once verified, unless
the user limits the scope. It does not authorize merging. A planning or approval
request alone authorizes none of those publication actions. Environment approval
requirements still apply.

Once verified, commit the final evidence, push the feature branch, and open or
update one PR targeting `develop`. Include the identifier, a relative link to
the plan, behavior changes, and validation results. Record the PR URL in the
plan and commit/push that metadata update. Leave status `verified` until merge.

## Finalize after merge

When finalization is requested, confirm the feature PR was merged into `develop`
using remote evidence. A closed, unmerged PR is not complete.

Create a documentation branch from updated `develop`, such as
`docs/mvp-1-archive-plan`. Record the feature PR URL, merge commit, and merge date;
set `complete`; move the document with `git mv` to
`doc/features/done/MVP-<number>.md`. Repair links affected by the move. Commit,
push, and open a small follow-up PR into `develop`. Finalization authorizes these
documentation publication actions, but not merging that PR. Report the archive
PR as pending until it merges; the archive reaches `develop` through that PR.

## Example requests

- “Review MVP-2 and prepare its implementation plan.”
- “Mark MVP-2 ready for implementation.”
- “Implement MVP-2.”
- “Record my manual verification results for MVP-2: …”
- “Finalize MVP-2 after its PR has merged.”
