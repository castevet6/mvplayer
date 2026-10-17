#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin"

cat >"$tmp/bin/gh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$1" == repo && "$2" == view ]]; then
	printf '%s\n' castevet6/mvplayer
	 exit 0
fi
if [[ "$1" != api ]]; then
	echo "unexpected gh command: $*" >&2
	exit 2
fi
for arg in "$@"; do
	case "$arg" in
		repos/castevet6/mvplayer/pulls/42) printf '%s\n' '{"number":42,"state":"open","merged_at":null,"html_url":"https://github.com/castevet6/mvplayer/pull/42","base":{"ref":"develop","sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"head":{"ref":"feature/test","sha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}'; exit 0 ;;
		repos/castevet6/mvplayer/pulls/42/reviews)
			if [[ "${MOCK_REVIEWED:-0}" == 1 ]]; then
				printf '%s\n' '[{"id":701,"user":{"login":"c6-review-bot[bot]"},"commit_id":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","body":"### 🤖 Claude review\n<!-- claude-review run=0123456789abcdef0123456789abcdef round=1 base=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa head=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb -->","submitted_at":"2026-10-18T00:00:00Z"}]'
			else
				printf '%s\n' '[]'
			fi
			exit 0 ;;
		repos/castevet6/mvplayer/issues/42/comments) printf '%s\n' '[]'; exit 0 ;;
		repos/castevet6/mvplayer/commits/*/check-runs)
			case "${MOCK_CHECKS:-none}" in
				success) printf '%s\n' '{"check_runs":[{"name":"ci","status":"completed","conclusion":"success"}]}' ;;
				failure) printf '%s\n' '{"check_runs":[{"name":"ci","status":"completed","conclusion":"failure"}]}' ;;
				*) printf '%s\n' '{"check_runs":[]}' ;;
			esac
			exit 0 ;;
		repos/castevet6/mvplayer/commits/*/status) printf '%s\n' '{"statuses":[]}'; exit 0 ;;
		repos/castevet6/mvplayer/pulls/comments/700) printf '%s\n' 'https://api.github.com/repos/castevet6/mvplayer/pulls/42'; exit 0 ;;
		repos/castevet6/mvplayer/pulls/42/comments/700/replies) printf '%s\n' '{"id":701,"in_reply_to_id":700,"html_url":"https://github.com/castevet6/mvplayer/pull/42#discussion_r701"}'; exit 0 ;;
	esac
done
if [[ "$*" == *"query("* ]]; then
	if [[ "$*" != *'reviewThreads(first:100,after:$endCursor){pageInfo{hasNextPage endCursor} nodes{'* ]]; then
		echo 'review thread pagination cursor is not the first pageInfo in the query' >&2
		exit 2
	fi
	printf '%s\n' '{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":null}}}}}}'
	exit 0
fi
echo "unexpected gh API request: $*" >&2
exit 2
EOF
chmod +x "$tmp/bin/gh"

export PATH="$tmp/bin:$PATH"
status="$($repo_root/scripts/pr.sh status 42 --json)"
jq -e '.number == 42 and .state == "open" and .completed_review_rounds == 0 and .checks.passed == null and (.unresolved_threads|length) == 0' <<<"$status" >/dev/null
status="$(MOCK_CHECKS=success MOCK_REVIEWED=1 "$repo_root/scripts/pr.sh" status 42 --json)"
jq -e '.checks.passed == true and .changes_after_last_review == false and .completed_review_rounds == 1' <<<"$status" >/dev/null
summary="$(MOCK_CHECKS=success MOCK_REVIEWED=1 "$repo_root/scripts/pr.sh" status 42)"
[[ "$summary" == *'changed since review: false'* && "$summary" == *'checks passed: true'* ]]
status="$(MOCK_CHECKS=failure "$repo_root/scripts/pr.sh" status 42 --json)"
jq -e '.checks.passed == false' <<<"$status" >/dev/null
summary="$(MOCK_CHECKS=failure "$repo_root/scripts/pr.sh" status 42)"
[[ "$summary" == *'checks passed: false'* ]]

printf '%s\n' 'Reviewed the requested behavior and validation passed.' >"$tmp/reply.md"
reply="$($repo_root/scripts/pr.sh reply 42 700 --body-file "$tmp/reply.md")"
jq -e '.in_reply_to_id == 700' <<<"$reply" >/dev/null

if "$repo_root/scripts/pr.sh" status 0 >/dev/null 2>&1; then
	echo 'invalid PR number was accepted' >&2
	exit 1
fi
if "$repo_root/scripts/pr.sh" resolve 42 PRRT_unknown >/dev/null 2>&1; then
	echo 'unknown review thread was accepted' >&2
	exit 1
fi

echo 'PR workflow helper checks passed.'
