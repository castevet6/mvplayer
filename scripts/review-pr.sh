#!/usr/bin/env bash
# scripts/review-pr.sh <pr-number>
#
# One-shot Claude review of a pull request.
# - Claude only READS (Read/Grep/Glob). It cannot edit, run commands, push or merge.
# - This script posts the result as ONE GitHub review with one inline comment
#   per finding, authored by the reviewer GitHub App (bot identity).
#
# Usage:
#   ./scripts/review-pr.sh 42            # review and post
#   DRY_RUN=1 ./scripts/review-pr.sh 42  # review, print payload, post nothing
set -euo pipefail

PR="${1:?usage: review-pr.sh <pr-number>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel)"
cd "$REPO_ROOT"

REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)
SHA=$(gh pr view "$PR" --json headRefOid -q .headRefOid)

WORK=$(mktemp -d)
WT="$WORK/worktree"
cleanup() {
  git -C "$REPO_ROOT" worktree remove --force "$WT" 2>/dev/null || true
  rm -rf "$WORK"
}
trap cleanup EXIT

# --- 1. Check out the PR head in a temporary worktree ----------------------
# Claude reads the PR's version of the code; your own checkout is untouched.
git fetch --quiet origin "pull/$PR/head"
git worktree add --quiet --detach "$WT" "$SHA"

# --- 2. Run Claude (read-only) and get structured findings ------------------
SCHEMA='{
  "type": "object",
  "properties": {
    "summary":  { "type": "string" },
    "findings": { "type": "array", "items": {
      "type": "object",
      "properties": {
        "path":     { "type": "string" },
        "line":     { "type": "integer" },
        "severity": { "type": "string", "enum": ["blocking", "suggestion", "nit"] },
        "body":     { "type": "string" }
      },
      "required": ["path", "line", "severity", "body"]
    }}
  },
  "required": ["summary", "findings"]
}'

PROMPT='You are reviewing a pull request. The PR metadata and diff are on stdin.
The working directory is a checkout of the PR head, so you can Read/Grep the code for context.
Follow the review guidelines in REVIEW.md if it exists.

Report each distinct issue as a separate finding:
- path: file path as shown in the diff
- line: line number in the NEW version of the file. It MUST be a line that appears
  in the diff (an added or context line). Use Read to verify the line number.
- severity: blocking (bug, security, data loss, broken behaviour), suggestion, or nit
- body: what is wrong, why it matters, and a concrete suggested fix.

Put the overall assessment in summary. If there are no issues, return an empty findings list.
Do not modify any files.'

(
  cd "$WT"
  { gh pr view "$PR" --json title,body,files; echo; gh pr diff "$PR"; } | claude -p "$PROMPT" \
    --allowedTools "Read,Grep,Glob" \
    --disallowedTools "Edit,Write,Bash" \
    --output-format json --json-schema "$SCHEMA"
) > "$WORK/claude.json"

# --- 3. Build one review payload: summary + one inline comment per finding --
jq --arg sha "$SHA" '.structured_output | {
    commit_id: $sha,
    event: "COMMENT",
    body: ("### 🤖 Claude review\n<!-- claude-review -->\n\n" + .summary),
    comments: [.findings[] | {
      path, line, side: "RIGHT",
      body: ("🤖 **Claude · \(.severity)**\n<!-- claude-review -->\n\n" + .body)
    }]
  }' "$WORK/claude.json" > "$WORK/payload.json"

if [[ "${DRY_RUN:-0}" == "1" ]]; then
  cat "$WORK/payload.json"
  echo "DRY_RUN: nothing posted." >&2
  exit 0
fi

# --- 4. Post as the bot ------------------------------------------------------
BOT_TOKEN=$("$SCRIPT_DIR/bot-token.sh")

if GH_TOKEN="$BOT_TOKEN" gh api "repos/$REPO/pulls/$PR/reviews" \
     --method POST --input "$WORK/payload.json" >/dev/null; then
  echo "Posted Claude review on PR #$PR ($(jq '.comments | length' "$WORK/payload.json") inline findings)."
else
  # GitHub rejects the whole review if any line is outside the diff.
  echo "Inline review rejected, posting as a single comment instead." >&2
  jq -r '.structured_output |
    "### 🤖 Claude review\n<!-- claude-review -->\n\n" + .summary + "\n\n" +
    ([.findings[] | "- **[\(.severity)]** `\(.path):\(.line)` \(.body)"] | join("\n"))' \
    "$WORK/claude.json" > "$WORK/fallback.md"
  GH_TOKEN="$BOT_TOKEN" gh api "repos/$REPO/issues/$PR/comments" \
    -F body=@"$WORK/fallback.md" >/dev/null
  echo "Posted Claude review on PR #$PR as a single comment."
fi