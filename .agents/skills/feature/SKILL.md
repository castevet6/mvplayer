---
name: feature
description: Start or manage a planned MVPLAYER feature with an MVP document, approval, implementation, review, and closure.
---

# MVPLAYER features

This is an explicitly invoked entry point for the MVP lifecycle. Select an
existing identifier or create a uniquely numbered document for a new request.
Starting a new feature begins planning; it does not approve implementation.
Keep the selected feature context for subsequent requests and merge notification.
Ordinary maintenance and reviewed changes use the paths in `AGENTS.md`.

Read [the feature workflow](../../../doc/FEATURE-WORKFLOW.md). It is the
repository authority for statuses, approval, branch naming, publication
authorization, and archiving. Apply the stage requested by the user; planning
or approval alone does not request implementation.

Locate the identifier's document under `doc/features/` or `doc/features/done/`.
For a new request, use `doc/templates/feature.md`. Read the whole document before
editing it and inspect relevant code when planning or implementing.

- **Plan:** Produce a reviewable plan and repeatable acceptance checks. Finish
  with `planned` and unresolved questions clearly recorded.
- **Approve:** Record explicit user approval of the current scope and set
  `ready`. If material questions prevent a concrete plan, explain what remains.
- **Implement:** Apply the approval gate in `AGENTS.md` before code changes.
  Prepare or resume the feature branch, implement the approved scope, and keep
  the document current. Finish at `implemented` while required checks remain.
- **Verify and publish:** Record actual evidence, then set `verified` only when
  all required checks passed. Use the workflow's authorization rules to commit,
  push, and open or update the feature PR into `develop`.
- **Finalize:** When the user reports an authorized feature PR merged, confirm
  remote merge evidence, archive the plan, and publish one documentation commit
  directly on `develop` under the workflow's authorization rules. Report the
  commit and whether the push succeeded.

Report the document path, current status, work performed, verification results,
and any pending user action or PR. Keep approval and verification evidence
truthful when a stage cannot be completed.
