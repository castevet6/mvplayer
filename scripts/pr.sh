#!/usr/bin/env bash
# Read PR state and safely reply to or resolve review threads.
set -euo pipefail

usage() {
	cat >&2 <<'EOF'
usage:
  scripts/pr.sh status <pr-number> [--json]
  scripts/pr.sh reply <pr-number> <comment-id> --body-file <file>
  scripts/pr.sh resolve <pr-number> <thread-node-id>
EOF
}

fail() {
	echo "pr.sh: $*" >&2
	exit 2
}

need_tools() {
	command -v gh >/dev/null || fail "gh is required"
	command -v jq >/dev/null || fail "jq is required"
}

valid_pr() {
	[[ "$1" =~ ^[1-9][0-9]*$ ]] || fail "PR number must be a positive integer"
}

repo_slug() {
	gh repo view --json nameWithOwner --jq .nameWithOwner
}

load_status() {
	local pr="$1" repo owner name pr_json reviews issues threads checks statuses bot
	local graphql_query
	repo="$(repo_slug)"
	owner="${repo%%/*}"
	name="${repo#*/}"
	bot="${REVIEW_BOT_LOGIN:-c6-review-bot[bot]}"
	pr_json="$(gh api "repos/$repo/pulls/$pr")"
	reviews="$(gh api --paginate "repos/$repo/pulls/$pr/reviews" | jq -s 'add')"
	issues="$(gh api --paginate "repos/$repo/issues/$pr/comments" | jq -s 'add')"
	graphql_query='query($owner:String!,$name:String!,$number:Int!,$endCursor:String){repository(owner:$owner,name:$name){pullRequest(number:$number){reviewThreads(first:100,after:$endCursor){pageInfo{hasNextPage endCursor} nodes{id isResolved path line comments(first:100){nodes{databaseId author{login} body createdAt} pageInfo{hasNextPage endCursor}}}}}}}'
	threads="$(gh api graphql --paginate -F owner="$owner" -F name="$name" -F number="$pr" -f query="$graphql_query" | jq -s '[.[].data.repository.pullRequest.reviewThreads.nodes[]?]')"
	while IFS= read -r thread_id; do
		[[ -n "$thread_id" ]] || continue
		cursor="$(jq -r --arg id "$thread_id" '.[] | select(.id == $id) | .comments.pageInfo.endCursor' <<<"$threads")"
		comments_query='query($threadId:ID!,$endCursor:String){node(id:$threadId){... on PullRequestReviewThread{comments(first:100,after:$endCursor){nodes{databaseId author{login} body createdAt} pageInfo{hasNextPage endCursor}}}}}'
		comment_pages="$(gh api graphql --paginate -F threadId="$thread_id" -F endCursor="$cursor" -f query="$comments_query")"
		more_comments="$(jq -s '[.[].data.node.comments.nodes[]?]' <<<"$comment_pages")"
		threads="$(jq --arg id "$thread_id" --argjson more "$more_comments" 'map(if .id == $id then .comments.nodes += $more | .comments.pageInfo.hasNextPage = false else . end)' <<<"$threads")"
	done < <(jq -r '.[] | select(.comments.pageInfo.hasNextPage) | .id' <<<"$threads")
	checks="$(gh api --paginate "repos/$repo/commits/$(jq -r .head.sha <<<"$pr_json")/check-runs?per_page=100" | jq -s '[.[].check_runs[]? | {name,status,conclusion}]')"
	statuses="$(gh api --paginate "repos/$repo/commits/$(jq -r .head.sha <<<"$pr_json")/status?per_page=100" | jq -s '[.[].statuses[]? | {context,state,description}]')"

	jq -n \
		--arg repo "$repo" --argjson pr "$pr" --arg bot "$bot" \
		--argjson pull "$pr_json" --argjson reviews "$reviews" \
		--argjson issues "$issues" --argjson threads "$threads" \
		--argjson checks "$checks" --argjson statuses "$statuses" '
		def marker($body):
			{run_id: (([$body | capture("run=(?<v>[a-fA-F0-9-]+)")? | .v][0]) // null),
			 round: (([$body | capture("round=(?<v>[0-9]+)")? | .v][0] | tonumber?) // null),
			 reviewed_head: (([$body | capture("head=(?<v>[a-fA-F0-9]{40})")? | .v][0]) // null)};
		([ $reviews[] | select(.user.login == $bot and ((.body // "") | contains("<!-- claude-review"))) |
		  (marker(.body // "") + {source:"review", id:(.id|tostring), commit_id:.commit_id, summary:.body, created_at:.submitted_at}) ] +
		 [ $issues[] | select(.user.login == $bot and ((.body // "") | contains("<!-- claude-review")) and ((.body // "") | startswith("### 🤖 Claude review"))) |
		  (marker(.body // "") + {source:"fallback", id:(.id|tostring), commit_id:null, summary:.body, created_at:.created_at}) ]) as $rounds |
		($rounds | map(if .run_id == null then . + {key:(.source + ":" + .id)} else . + {key:("run:" + .run_id)} end)
		 | unique_by(.key) | sort_by(.created_at)) as $unique |
		($unique | last | .reviewed_head // .commit_id // null) as $last_head |
		{repository:$repo, number:$pr, state:(if $pull.merged_at != null then "merged" else $pull.state end), url:$pull.html_url,
		 base_ref:$pull.base.ref, base_sha:$pull.base.sha, head_ref:$pull.head.ref, head_sha:$pull.head.sha,
		 merged_at:$pull.merged_at, merge_sha:($pull.merge_commit_sha // null),
		 completed_review_rounds:($unique|length), latest_reviewed_sha:$last_head,
		 changes_after_last_review:(if $last_head == null then null else $last_head != $pull.head.sha end),
		 reviews:$unique,
		 review_threads:([$threads[] | {id,is_resolved:.isResolved,path,line,comments:[.comments.nodes[] | {id:.databaseId,author:.author.login,body,created_at:.createdAt}],comments_truncated:.comments.pageInfo.hasNextPage}]),
		 unresolved_threads:([$threads[] | select(.isResolved == false) | {id,path,line,comments:[.comments.nodes[] | {id:.databaseId,author:.author.login,body,created_at:.createdAt}],comments_truncated:.comments.pageInfo.hasNextPage}]),
		review_threads_truncated:false,
		 checks:{check_runs:$checks,status_contexts:$statuses,
		  state:(if (($checks|length) + ($statuses|length)) == 0 then "unknown"
		   elif ([$checks[] | select(.status == "completed" and ((.conclusion // "") | ascii_downcase | . != "success" and . != "skipped" and . != "neutral"))]|length > 0) or
		        ([$statuses[] | select(.state == "failure" or .state == "error")]|length > 0) then "failed"
		   elif ([$checks[] | select(.status != "completed")]|length > 0) or
		        ([$statuses[] | select(.state == "pending")]|length > 0) then "pending"
		   else "passed" end),
		  passed:(if (($checks|length) + ($statuses|length)) == 0 then null
		   elif ([$checks[] | select(.status == "completed" and ((.conclusion // "") | ascii_downcase | . != "success" and . != "skipped" and . != "neutral"))]|length > 0) or
		        ([$statuses[] | select(.state == "failure" or .state == "error")]|length > 0) then false
		   elif ([$checks[] | select(.status != "completed")]|length > 0) or
		        ([$statuses[] | select(.state == "pending")]|length > 0) then null
		   else true end)}}
	'
}

cmd="${1:-}"
case "$cmd" in
status)
	[[ $# -ge 2 && $# -le 3 ]] || { usage; exit 2; }
	pr="$2"
	valid_pr "$pr"
	need_tools

	result="$(load_status "$pr")" || fail "could not read PR state; check network-capable execution and gh authentication"
	if [[ "${3:-}" == "--json" ]]; then
		jq . <<<"$result"
	elif [[ $# == 2 ]]; then
		jq -r '
			"PR #\(.number) \(.state | ascii_upcase): \(.url)",
			"head \(.head_ref) \(.head_sha); base \(.base_ref) \(.base_sha)",
			"Claude rounds: \(.completed_review_rounds); latest reviewed SHA: \(.latest_reviewed_sha // "unknown"); changed since review: \(if .changes_after_last_review == null then "unknown" else (.changes_after_last_review|tostring) end)",
			"checks: \(.checks.state)",
			(if (.unresolved_threads|length)==0 then "unresolved threads: none" else
			 "unresolved threads:\n" + ([.unresolved_threads[] | "- \(.path):\(.line // "general") [\(.id)] " + ([.comments[] | "\(.author): \(.body | split("\n") | first)"] | join("; "))] | join("\n")) end)
		' <<<"$result"
	else
		usage
		exit 2
	fi
	;;
reply)
	[[ $# == 5 && "$4" == "--body-file" ]] || { usage; exit 2; }
	pr="$2"
	comment="$3"
	body_file="$5"
	valid_pr "$pr"
	[[ "$comment" =~ ^[1-9][0-9]*$ ]] || fail "comment ID must be a positive integer"
	[[ -r "$body_file" ]] || fail "body file is not readable"
	need_tools
	repo="$(repo_slug)"
	comment_pr="$(gh api "repos/$repo/pulls/comments/$comment" --jq .pull_request_url)"
	expected="https://api.github.com/repos/$repo/pulls/$pr"
	[[ "$comment_pr" == "$expected" ]] || fail "comment $comment does not belong to PR #$pr"
	body="$(cat "$body_file")"
	[[ -n "${body//[[:space:]]/}" ]] || fail "reply body is empty"
	gh api "repos/$repo/pulls/$pr/comments/$comment/replies" --method POST -f "body=$body" --jq '{id,in_reply_to_id,html_url}'
	;;
resolve)
	[[ $# == 3 ]] || { usage; exit 2; }
	pr="$2"
	thread="$3"
	valid_pr "$pr"
	[[ "$thread" == PRRT_* ]] || fail "thread ID must be a pull request review thread node ID"
	need_tools
	status="$(load_status "$pr")" || fail "could not read PR state; check network-capable execution and gh authentication"
	jq -e --arg id "$thread" 'any(.unresolved_threads[]; .id == $id)' <<<"$status" >/dev/null || fail "thread $thread is not an unresolved thread on PR #$pr"
	gh api graphql -F threadId="$thread" -f query='mutation($threadId:ID!){resolveReviewThread(input:{threadId:$threadId}){thread{id isResolved}}}'
	;;
*)
	usage
	exit 2
	;;
esac
