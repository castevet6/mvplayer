# Review guidelines

Used by `scripts/review-pr.sh`. Edit freely to tune what the reviewer focuses on.

## Focus on
- Correctness: logic errors, off-by-one, wrong conditions, unhandled edge cases
- Error handling: swallowed errors, missing null/empty checks, failure paths
- Security: injection, secrets in code, unsafe input handling, overly broad permissions
- Data integrity: anything that could lose or corrupt user data
- Tests: changed behaviour without tests, tests that don't actually assert anything

## Lower priority
- Readability and naming: only when it genuinely hurts understanding
- Style and formatting: ignore anything a formatter or linter would catch

## Severity
- **blocking**: bug, security issue, data loss, or broken behaviour. Must be fixed before merge.
- **suggestion**: a real improvement, but the PR works without it.
- **nit**: minor polish. Use sparingly — at most a few per review.

## Project conventions
<!-- Add your project's conventions here, e.g. language/framework rules,
     folder structure, how errors are reported, what not to touch. -->