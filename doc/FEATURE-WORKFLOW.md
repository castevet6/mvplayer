# Feature workflow

Planned MVP requests, approved plans, and verification evidence are versioned
with the code. Each planned feature has one document: `doc/features/MVP-<number>.md`.
Use [the template](templates/feature.md) for new requests. Keep the identifier
unique across active and archived features.

## Enter and continue the lifecycle

Start a new planned feature explicitly with the `feature` skill
(`$feature`, or select it in the client's skill interface). A new invocation
starts planning and creates or selects the feature document; it grants no
implementation approval. `/feature` is shorthand for this workflow in
conversation, not a separately installed slash command.

Once a feature is selected, follow-up requests about its plan, approval,
implementation, verification, PR, or merge continue that feature without
requiring another invocation. An explicit request to resume an existing MVP
identifier also selects its document; inspect its approval and current status
before acting. Other tasks do not inherit the feature lifecycle merely because
they occur in the same chat or mention a feature.

The delivery paths in `AGENTS.md` govern ordinary maintenance and reviewed
changes. A request for a fix, refactor, or new behavior does not automatically
create an MVP document. Recommend this lifecycle when formal planning would
help and let the user choose whether to enter it.

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

Prepare planning edits on `develop` or the feature's existing branch while
preserving unrelated work. Planning alone does not authorize publication.
Carry the approved plan into the implementation branch.

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

When the user reports that an authorized feature PR has merged, confirm the
merge into `develop` using remote evidence. This message triggers finalization;
no separate archive request is needed. A closed, unmerged PR is not complete.

Update local `develop` from the remote while preserving unrelated work. Inspect
divergent commits; continue past patch-equivalent duplicates, but stop and report
unique local commits without rewriting shared history. Record the feature PR
URL, merge commit, and merge date; set `complete`; move the document with `git mv` to
`doc/features/done/MVP-<number>.md`. Repair links affected by the move.

Check the documentation diff and affected links, then commit the finalization
as one small documentation commit directly on `develop` and push normally.
Finalization authorizes this documentation commit and direct push; it requires
no additional branch, PR, or PR review round. Preserve the published merge
commit and shared history. If branch protection rejects the direct push, report
the restriction and leave the commit local rather than creating a follow-up PR
without a user request. Report the finalization commit and whether it reached
remote `develop`.

## Example requests

- “Use $feature to plan a new playlist feature.”
- “Review MVP-2 and prepare its implementation plan.”
- “Mark MVP-2 ready for implementation.”
- “Implement MVP-2.”
- “Record my manual verification results for MVP-2: …”
- “Finalize MVP-2 after its PR has merged.”
