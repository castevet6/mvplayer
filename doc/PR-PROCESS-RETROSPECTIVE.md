# PR process retrospective: MVP-1

## Outcome and evidence

MVP-1 exercised the intended split: Codex implemented and orchestrated the PR;
Claude reviewed through `scripts/review-pr.sh`; the human merged PR #1. Two
Claude rounds produced six findings: four in round one and two in round two.
All were fixed, answered, and resolved. Neither review reported a blocking
product bug. Tests and vet passed after the last fixes.

The second-round fixes were published after the last Claude review, as allowed
by the two-round limit. The final head therefore had validated changes that
Claude had not reviewed. Resolved threads and passing checks are distinct from
a clean review of that final head.

PR #1 merged into `develop` as
`65e6c435f93a2f3df3c50d88a46ae25e13b0a319` on 2026-10-17. Finalization
published the revised workflow and the archived plan directly to `develop` in
`f0fbc2d` and `d8d7f51`. No archive PR was created.

This retrospective uses the session history, current repository instructions,
and review scripts. Observed problems below are distinguished from additional
gaps found by inspecting the script. Recommendations are proposals, except
where explicitly identified as already implemented.

## What worked

- **Clear division of work.** Claude supplied the independent review; Codex
  assessed findings, made fixes, validated them, and handled GitHub responses.
  Codex did not add another independent review that duplicated Claude's role.
- **Review isolation.** Claude inspected a detached checkout of the PR head,
  with configured read tools and edit/command tools disallowed. The working
  checkout and local edits were preserved.
- **Useful, bounded feedback.** Findings covered a script permission issue,
  footer discoverability and accuracy, environment-test correctness, and module
  metadata. Severity rules and the two-round limit kept follow-up bounded.
- **Public evidence.** The bot posted inline findings under its own identity;
  Codex replied with fixes and validation and resolved addressed threads.
  Tokens and private keys were kept out of comments and logs.
- **Checks caught a regression.** Reordering hints initially hid `Tab switch`
  at the minimum width. Existing tests caught that before publication.
- **Truthful completion evidence.** Codex verified the remote merge rather
  than treating a closed PR or a local branch as proof of merge.
- **The process improved during use.** The review script's executable bit was
  fixed, GitHub network guidance was added, and finalization was simplified.

## Observed problems and proposed improvements

| Finding | What happened | Improvement |
| --- | --- | --- |
| Sandbox failure was reported as invalid credentials | `gh auth status` failed inside the restricted environment but succeeded with the same credentials outside it. Codex initially recommended an authentication fix without establishing the cause. | Check execution context first. Test GitHub access with network-capable execution before diagnosing credentials. This rule is now documented; apply it consistently. |
| Escalation was repetitive and inconsistently applied | Later GitHub calls still ran without required network access, while many narrowly different calls caused repeated escalation prompts. | Establish the permitted execution path at the start. Reuse existing approvals and batch compatible reads. Explain that task authorization and sandbox execution permission are separate. Request only the command scope actually needed. |
| The documented script command did not work | `./scripts/review-pr.sh` failed because its executable bit was absent. Running through `bash` bypassed that symptom until the review caught it. | Already fixed. Add a cheap tracked-mode/prerequisite check so workflow tooling works before an expensive review begins. |
| Go cache location was unwritable | `go mod tidy` initially failed on the default build cache. Redirecting `GOCACHE` to `/tmp/mvplayer-gocache` worked. | Use one documented writable cache setting for sandboxed Go commands. Treat this as an execution setup issue, separate from dependency or code failures. |
| GitHub API calls were guessed | Codex tried an unavailable GraphQL mutation, an incorrect REST reply path, and a nonexistent GraphQL field. | Put working thread-list, reply, and resolve operations behind a small maintained helper. A reply uses `POST /repos/{owner}/{repo}/pulls/{number}/comments/{comment_id}/replies`. Validate one operation before repeating it across all findings. |
| Review state took many ad hoc queries | Round counting, inline comments, conversation comments, and thread resolution required repeated manual retrieval. | Produce one compact status report containing PR state, head SHA, completed rounds, latest reviewed SHA, and all unresolved threads. Paginate results and include other reviewers' threads. |
| Following suggestions introduced fresh inaccuracies | The first fix removed the Browser footer's Queue-focus qualifier while retaining `d remove`, and also added `? help` during filter editing. Both became second-round findings. | Codex should translate a suggestion into intended behavior, then check affected states before editing. Preserve the review concern without copying an example blindly or expanding the fix unnecessarily. |
| Initial assertions checked visibility more than truth | Help and quit became visible, but the hints did not match the actions accepted in each mode. | Check the state/action relationship: Browser, Queue, filter editing, and minimum/normal widths. Assert that unavailable actions are omitted as well as available actions being visible. Keep this focused rather than adding broad snapshot maintenance. |
| Progress messages mostly described waiting | The script emitted no intermediate stages while Claude ran. Codex had little information beyond “still running.” | Print short stage transitions and elapsed time: setup, review, validation, publication. Preserve safe failure diagnostics; never print credentials. |
| The merge/finalization handoff was unclear | The user requested a merge, the repository assigned merges to a human, and “PR is now merged” led to an announced archive action followed by another authorization question. | State the human merge handoff when the PR becomes ready. Define one clear post-merge trigger and its authorization once, so Codex can carry out routine closure without renegotiating each step. |
| Archive policy created unnecessary work | Finalization originally required a new branch, PR, and another review process for a status update and file move. | Already changed: one small archive commit directly on `develop`, with merge evidence and repaired links. No archive PR or review round. |
| Local history made finalization awkward | Local `develop` had a patch-equivalent copy of a workflow commit. Codex checked equivalence and rebased it away; workflow edits were first committed on the merged feature branch and then cherry-picked. | Inspect local/remote ancestry and worktree state before selecting a commit location. Define patch-equivalent duplicates explicitly; stop for unique divergent work. Prefer committing workflow changes on the intended destination from the outset. |
| Instructions contain overlap and stale text | PR rules span AGENTS.md, the PR skill, the feature skill, and the workflow. AGENTS.md still claims Git history is unavailable. | Give each rule one authoritative home and use short pointers elsewhere. Remove stale observations and distinguish lifecycle rules from execution mechanics. |

The four first-round findings were all addressed: executable script mode,
footer visibility, a genuinely unset `NO_COLOR` case, and direct dependency
metadata. The two second-round findings were also addressed: omit Queue-only
deletion from Browser hints and omit help from filter-editing hints.

## Additional script gaps found during this retrospective

These were not observed failures in PR #1. They are opportunities to make the
next run more predictable.

1. **Pin all review inputs to one commit.** The script captures a head SHA and
   checks it out, but later calls fetch the live PR diff and metadata. A push
   during review could make the code, diff, and posted commit disagree. Capture
   the base and head together, derive the diff from those commits, and recheck
   the remote head before publication. Report a changed head explicitly.
2. **Track rounds in GitHub and enforce the limit in the script.** The current
   script has no round guard. Include the head SHA and round in a machine-readable
   review marker. Count completed bot review publications, including fallback
   comments, once each; author replies and pre-publication failures are not
   additional completed rounds. Record failed attempts separately. Serialize
   concurrent runs for the same PR to avoid duplicate reviews.
3. **Supply previous findings to round two.** The current input contains PR
   metadata and its full diff, but no prior review or disposition. Supply prior
   findings, Codex responses, and changes since the reviewed commit, alongside
   the current scope. Claude can then verify fixes and look for regressions.
4. **Validate output before posting.** Check that Claude completed successfully
   and returned the expected structured object, valid severities, paths, and
   usable line numbers. Invalid or missing output must be a failed review, not
   an empty clean review. Validate the bot token response without logging it.
5. **Separate publication failures from invalid inline locations.** The script
   currently falls back to a conversation comment after any review-post failure.
   Use fallback for location-validation errors; identify authentication,
   authorization, rate, and service failures separately. Verify whether a timed-out
   publication succeeded before retrying, to avoid duplicate posts.
6. **Keep recoverable diagnostics.** Cleanup removes the temporary JSON and
   payload even after failure. Keep a local, ignored, credential-free result
   long enough to diagnose or retry publication without rerunning Claude.

## Proposed lean process

1. **Establish readiness once.** Codex checks feature approval, branch/base,
   local work, execution permissions, required tools, and writable caches.
   Reuse known working session setup. Ask only when a genuine permission or
   scope decision is missing.
2. **Publish one feature PR.** Run required checks, push, create or update the
   PR, and give a concise behavior/validation description. Codex remains the
   author and orchestrator; Claude remains the reviewer.
3. **Run the review against recorded inputs.** The helper determines the next
   allowed round, pins the reviewed commits, runs Claude, validates output,
   and publishes one review or one supported fallback artifact.
4. **Address findings as a batch.** Codex reads all open feedback, assesses
   severity and scope, fixes valid issues, runs affected and required checks,
   pushes once where practical, then replies with commit and validation.
   Resolve fixed threads; leave disputed or blocked findings visible with reasons.
5. **Complete at most two rounds.** Round two verifies fixes and checks for
   regressions. Fix its findings, validate, and push without automatically
   starting round three. If material behavior changed after the last review,
   make that limitation explicit for the human's merge decision.
6. **Hand off the merge clearly.** Report URL, head SHA, latest reviewed SHA,
   rounds used, checks, remaining findings, and changes after review. Say
   “ready for human merge” only when there are no known unresolved blockers;
   it does not mean the bot approved every change at the final SHA.
7. **Close the feature with one small commit.** Proposed authorization: include
   routine post-merge closure in the authorized feature lifecycle. When the
   user reports a merge or requests finalization, Codex verifies the remote
   evidence, updates the plan, archives it, checks links, and pushes one small
   documentation commit directly to `develop`. Preserve the published merge
   commit. If protection rejects the push or unique local history diverges,
   report the concrete issue rather than inventing another PR workflow.

The direct-commit mechanism in step 7 is already adopted. Extending the trigger
to a merge notification without a separate finalization request remains a
proposal. Keep feature records compact: approval, scope, validation, PR, and
merge evidence. GitHub holds the detailed discussion; the plan need not become
a transcript of every command or review exchange.

## Implementation priorities

| Priority | Change | Completion criterion |
| --- | --- | --- |
| First | Define handoff, closure authorization, and authoritative instruction locations | One unambiguous lifecycle; routine review fixes and closure do not trigger repeated conversational approval gates. |
| First | Add a compact PR status/thread helper using known working operations | One invocation reports review state and can reply to the correct thread without API guesswork. |
| First | Harden the existing review script | Pinned input, validated output, stage messages, durable round identity, and no duplicate run or misleading success. |
| Next | Give round two previous findings and fix context | Its input shows the earlier review, dispositions, and intervening changes. |
| Next | Add focused workflow checks | Fixtures exercise round counting, malformed output, stale heads, fallback publication, and retries; checks confirm script executable mode. No real PR or token is required for routine verification. |
| Ongoing | Keep finalization and documentation small | One archive commit, no extra PR, no merge-history rewrite, no repeated application tests for documentation-only closure. |

Success means the user makes scope decisions and performs the merge, while
Codex reliably handles execution, review follow-up, and closure. Measure useful
outcomes: avoidable approval prompts, failed command attempts, duplicate review
posts, repeated findings, and whether the archived feature reaches `develop`.
Use those signals to simplify the process further rather than adding more
documents or review stages.
