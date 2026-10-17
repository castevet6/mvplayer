#!/usr/bin/env bash
# scripts/bot-token.sh
# Prints a 1-hour installation token for the reviewer GitHub App.
# Reads APP_ID, INSTALLATION_ID and KEY_FILE from ~/.config/pr-reviewer/config
set -euo pipefail
source "$HOME/.config/pr-reviewer/config"

b64url() { openssl base64 -A | tr '+/' '-_' | tr -d '='; }

now=$(date +%s)
header=$(printf '{"alg":"RS256","typ":"JWT"}' | b64url)
payload=$(printf '{"iat":%d,"exp":%d,"iss":"%s"}' $((now - 60)) $((now + 540)) "$APP_ID" | b64url)
signature=$(printf '%s.%s' "$header" "$payload" \
  | openssl dgst -sha256 -sign "$KEY_FILE" -binary | b64url)
jwt="$header.$payload.$signature"

curl -fsS -X POST \
  -H "Authorization: Bearer $jwt" \
  -H "Accept: application/vnd.github+json" \
  "https://api.github.com/app/installations/$INSTALLATION_ID/access_tokens" \
  | jq -r .token