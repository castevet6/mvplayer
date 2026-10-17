---
name: github-pr
description: Create or update GitHub pull requests with the GitHub CLI, and address findings the user reports. Use for PR publication or user-requested comment follow-up; this skill does not conduct independent code reviews.
---

# GitHub pull requests

Use `gh` to create or update a pull request, then report its URL and current
state and leave it open. Read repository instructions first. For an MVPLAYER
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
   workflow. PR creation does not authorize merging; leave the PR open and wait
   for the user to explicitly say it can be merged or to request a merge.

## Review comments

The user will report whether comments or findings need attention. Do not
proactively poll, retrieve, or review PR comments. When the user reports findings
and asks for them to be addressed, retrieve the relevant comments with the
GitHub CLI as needed, fix each actionable finding in code, tests, or
documentation, and run the affected checks plus required repository-wide
checks. Do not broaden this into an independent code review, and do not reply to
reviewers unless the user asks for a response. Report what was fixed and leave
the PR open.

Do not merge automatically after findings are fixed or checks pass. Wait for the
user to explicitly say the PR can be merged or to request a merge. After that
signal, confirm the PR is mergeable and the required checks pass, then merge it.
If findings remain, checks fail, or the PR is not mergeable, explain what is
blocking the merge and leave the PR open. Report the final PR state clearly.
