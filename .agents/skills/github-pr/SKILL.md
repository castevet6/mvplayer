---
name: github-pr
description: Create or update GitHub pull requests with the GitHub CLI and handle their review comments. Use for PR publication or comment follow-up; this skill does not conduct independent code reviews.
---

# GitHub pull requests

Use `gh` to create or update a pull request, then report its URL and current
state. Read repository instructions first. For an MVPLAYER MVP feature, also
follow [the feature workflow](../../../doc/FEATURE-WORKFLOW.md) and use the
`mvplayer-feature` skill for its plan status and publication requirements.

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
   workflow. Do not merge during PR creation unless the user's instruction
   explicitly authorizes a merge condition that has been met.

## Review comments

When a requested PR task includes an existing PR, retrieve its reviews and
comments with `gh pr view` and `gh api` (including paginated inline review
comments and issue comments). Address every actionable finding in code, tests,
or documentation. Run the affected checks and required repository-wide checks.
Keep the PR open while findings remain. Do not broaden this into an independent
code review, and do not reply to reviewers unless the user asks for a response.

Treat an explicit user instruction that the PR may be merged once all comments
are fixed as conditional merge authorization. Merge only after every actionable
finding is fixed, required checks pass, and no review thread remains unresolved.
Otherwise report the remaining findings or checks and leave the PR open. Report
the PR state and any pending action clearly.
