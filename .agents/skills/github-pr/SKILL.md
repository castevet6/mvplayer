---
name: github-pr
description: Create or update GitHub pull requests, run the repository Claude review, and address review findings.
---

# GitHub pull requests

Use this skill for PR publication and review follow-up on both ordinary changes
and planned features. Read `AGENTS.md` for delivery-path selection. For an
active MVP feature, also read `doc/FEATURE-WORKFLOW.md` and its feature document.
`AGENTS.md` points here for PR mechanics; keep detailed round and API procedures
in this skill rather than duplicating them there.

## Access and authorization

At the first GitHub task in each Codex chat, check whether network-capable
`gh` execution is already available. If it is, use it without another prompt.
If it is not, request the needed execution approval once, choosing the
current-chat/session scope when the host offers that choice. After approval,
reuse that access for authorized `gh` work throughout this chat; do not ask
again for each ordinary PR command. If the host only offers command-scoped
approval, request the narrowest available GitHub CLI access and reuse whatever
authorization the host grants. Repository instructions cannot grant sandbox
access or GitHub permissions.

Once execution is network-capable, check `gh auth status` before diagnosing
credentials. Execution approval only allows the command to run; it does not
authenticate `gh`, expand token scopes, or authorize GitHub actions beyond the
user's task and the PR workflow below. Keep tokens and keys out of output and
comments.

Opening or updating a PR authorizes the review workflow and in-scope fixes,
checks, commits, pushes, and replies. Renew scope approval for material behavior
or dependency changes. A human merges every PR; Codex never merges, approves its
own PR, or dismisses the reviewer bot.

For ordinary work assigned to the change-with-review path, an implementation
request authorizes checks, focused commits, pushing the change branch, and
opening a PR into `develop`, unless the user limits publication. Use a
descriptive branch such as `fix/<description>` or `chore/<description>` based
on current remote `develop`. Preserve unrelated work when preparing it. This
path needs no MVP document or feature-plan approval. Planning or proposals
alone authorize no publication. For planned features, preserve the separate
approval and publication rules in `doc/FEATURE-WORKFLOW.md`.

## Create or update

1. Confirm repository, head and base, local changes, commit range, and required
   validation. Describe the actual diff without performing a separate Codex
   review.
2. Check authentication, then find an existing PR with
   `gh pr list --state all --head <branch>`. Update it or create one for the
   requested base. Use a body file for multiline descriptions.
3. Push only when authorized. Leave the PR open and run the Claude review before
   reporting it ready for human merge.

## Review and follow-up

Use `scripts/pr.sh` as the source of truth for PR state and thread operations.
Before starting a round, inspect all prior bot summaries, fallback comments,
reviews, and unresolved threads, including other reviewers. Read the full thread
context. `scripts/pr.sh status <number>` provides the concise state;
`scripts/pr.sh status <number> --json` includes checks, prior Claude rounds,
full thread context, and whether the head changed after review. Use
`scripts/pr.sh reply <number> <comment-id> --body-file <file>` for inline
replies and `scripts/pr.sh resolve <number> <thread-node-id>` only after the
thread is addressed. Check state is `passed`, `failed`, `pending`, or `unknown`;
pending work is not a failure or a completed validation. A status failure is
not a clean review.

Run `scripts/review-pr.sh <pr-number>` and wait for completion. It reviews a
pinned base/head pair in a detached worktree. Round one assesses the PR. Round
two receives the prior findings, replies, dispositions, and changes since the
last reviewed head, including an incremental diff when that commit is
available. At most two Claude reviews may be published per PR across sessions.
The runner serializes concurrent requests per PR and stores
permission-restricted run artifacts under ignored `build/pr-review/`. Failed
attempts before publication do not consume a round; retries use
`scripts/review-pr.sh --resume <run-directory>` to publish the saved result
without spending another Claude review. A published fallback counts as one
review. Run `make test-pr-workflow` when changing these scripts.

Fix all valid blocking findings, suggestions when clearly right, and trivial
nits. Verify each proposed fix against actual behavior and affected states.
Report scope changes or disputed findings and leave them unresolved with
evidence. Run affected and repository-required checks. Commit and push fixes in
scope, reply with the change and validation, and resolve fixed threads.

After round two, still fix and validate actionable findings, then push and reply
without starting round three. Report changes made after the last reviewed head
so the human can assess the remaining review gap.

## Handoff and feature closure

Report the PR URL, current head, latest reviewed head, review rounds, validation,
unresolved findings, and changes made after review. Say it is ready for human
merge only when no known blocker remains; this does not imply approval at an
unreviewed final head.

For a feature PR whose publication was authorized, a user message that it has
merged triggers remote verification and feature finalization. Follow
`doc/FEATURE-WORKFLOW.md`: record merge evidence, archive the feature plan, and
push one small documentation commit directly to `develop`. Do not create an
archive PR. If remote history has unique divergence or protection rejects the
push, preserve the work and report the concrete blocker.

For an ordinary change PR, verify and report the merge when notified. There is
no feature plan to archive or documentation finalization commit to create.
