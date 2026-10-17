---
name: github-pr
description: Create or update GitHub pull requests with the GitHub CLI, run the repository Claude review workflow, and address PR findings and open review comments. Use for PR publication, updates, or review-comment follow-up.
---

# GitHub pull requests

Use `gh` to create or update a pull request, then report its URL and current
state and leave it open. Read repository instructions first, including the
"Pull requests and review" policy in `AGENTS.md`. For an MVPLAYER
MVP feature, also follow [the feature workflow](../../../doc/FEATURE-WORKFLOW.md)
and use the `mvplayer-feature` skill for its plan status and publication
requirements.

## Create or update

1. Confirm the requested head and base branches, inspect the working tree and
   commit range, and run the checks required by the repository. Review the diff
   to describe the requested work accurately; do not perform an independent
   code review as part of PR creation.
2. Confirm `gh auth status` and repository context. Find an existing PR for the
   head branch with `gh pr list --state all --head <branch>`. Update that PR when one exists;
   otherwise create one with `gh pr create` and the requested base.
3. Write a concise title and body that state the behavior change and validation.
   Follow repository-specific requirements for feature identifiers, plan links,
   target branches, and evidence. Use `--body-file` when the body has multiple
   paragraphs or lists.
4. Push only when publication is authorized by the user and repository
   workflow. After opening or updating the PR, complete the review workflow
   below before reporting completion. Leave the PR open for a human to merge.

## Review and follow-up

Follow the authoritative review procedure in `AGENTS.md`, including its commands,
severity rules, two-round limit per PR across sessions, and reporting requirements.
Run `./scripts/review-pr.sh <pr-number>` and wait for completion. Claude performs
the review using `REVIEW.md`; Codex retrieves findings and open review comments,
fixes actionable issues, validates changes, commits and pushes fixes, and replies
with the outcome. This workflow authorizes those review responses without another
permission request. It does not request an independent Codex code review.

Read review threads as well as review summaries and conversation comments. Use
`gh api graphql` to retrieve paginated `reviewThreads` with `isResolved`, comments,
and reply context when needed; inline comment listings alone do not indicate
whether a thread is resolved. Account for previous bot reviews and fallback
comments before starting another round. Fixes after the second round still need
validation and publication; report them as changes made after the last review.

If the script or required checks fail, report the failure and remaining work.
Preserve feature approval gates when a finding requires a scope change. Leave
unresolved or disputed blockers visible with an explanation and evidence.

Never merge a PR, approve your own PR, or dismiss the bot's review. A human
performs merges.
