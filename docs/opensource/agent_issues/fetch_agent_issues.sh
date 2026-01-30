#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_DIR="$ROOT_DIR/snapshots"

# Defaults
TOP_N="${TOP_N:-10}"
FOCUS_ONLY="${FOCUS_ONLY:-1}"   # 1=only Codex/Kode/OpenCode/QwenCode
EXTRA="${EXTRA:-0}"             # 1=also fetch secondary agent repos
SLEEP_SECONDS="${SLEEP_SECONDS:-1}"

mkdir -p "$OUT_DIR"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing dependency: $1" >&2
    exit 1
  fi
}

require_cmd jq
require_cmd curl

sanitize_slug() {
  echo "$1" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/_/g' | sed -E 's/^_+|_+$//g'
}

fetch_repo() {
  local repo="$1"
  local name="$2"
  local slug
  slug="$(sanitize_slug "$name")"

  local fetched_at
  fetched_at="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

  local q="repo:${repo} is:issue is:open"

  echo "Fetching: $name ($repo) top=${TOP_N} ..."

  # GitHub Search API supports sort/order params.
  # NOTE: We use curl (not `gh api`) because some environments route `gh` to a GitHub Enterprise host
  # where search endpoints may not be available.
  local q_encoded
  q_encoded="$(printf '%s' "$q" | jq -sRr @uri)"
  local url="https://api.github.com/search/issues?q=${q_encoded}&sort=reactions&order=desc&per_page=${TOP_N}"

  local json_path="$OUT_DIR/${slug}.json"
  local headers_path
  headers_path="$(mktemp)"

  local -a curl_args
  curl_args=(
    -sS
    -H "Accept: application/vnd.github+json"
    -H "X-GitHub-Api-Version: 2022-11-28"
  )
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    curl_args+=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
  fi

  curl "${curl_args[@]}" -D "$headers_path" "$url" -o "$json_path"

  # Best-effort rate limit awareness (Search API is much stricter than core API).
  # If we are rate-limited, we still keep the raw response and write a readable failure stub.
  local remaining reset_at
  remaining="$(awk -F': ' 'tolower($1)=="x-ratelimit-remaining"{print $2}' "$headers_path" | tail -n 1 | tr -d '\r' || true)"
  reset_at="$(awk -F': ' 'tolower($1)=="x-ratelimit-reset"{print $2}' "$headers_path" | tail -n 1 | tr -d '\r' || true)"
  if [[ "${remaining:-}" == "0" ]]; then
    echo "Rate limit reached (remaining=0, reset=${reset_at})." >&2
  fi

  if ! jq -e '.items and (.items|type=="array")' "$json_path" >/dev/null 2>&1; then
    local msg status
    msg="$(jq -r '.message // empty' "$json_path" 2>/dev/null || true)"
    status="$(jq -r '.status // empty' "$json_path" 2>/dev/null || true)"

    cat > "$OUT_DIR/${slug}.md" <<EOF
# ${name} — Top Issues Snapshot (open, sorted by reactions)

- Fetched at (UTC): ${fetched_at}
- Repo: https://github.com/${repo}
- Query: \`${q} sort:reactions-desc\`
- Top N: ${TOP_N}

## Fetch failed

- Status: ${status}
- Message: ${msg}
$(if [[ -n "${remaining:-}" ]]; then echo "- RateLimit-Remaining: ${remaining}"; fi)
$(if [[ -n "${reset_at:-}" ]]; then echo "- RateLimit-Reset: ${reset_at}"; fi)

Raw response saved at: \`${slug}.json\`
EOF
    rm -f "$headers_path"
    return 0
  fi

  cat > "$OUT_DIR/${slug}.md" <<EOF
# ${name} — Top Issues Snapshot (open, sorted by reactions)

- Fetched at (UTC): ${fetched_at}
- Repo: https://github.com/${repo}
- Query: \`${q} sort:reactions-desc\`
- Top N: ${TOP_N}

> Notes
> - This is a **snapshot**, not an evaluation.
> - Body is truncated for readability; use the link for full context.

## Issues

EOF

  jq -r '
    def labels: (.[].name // empty) as $n | $n;
    def label_list: ( .labels // [] | map(.name) | if length>0 then ("`"+join("`, `")+"`") else "" end );
    def body_preview:
      ( .body // "" )
      | gsub("\r\n"; "\n")
      | gsub("\n{3,}"; "\n\n")
      | if length > 600 then (.[:600] + "…") else . end;

    .items
    | map({
        number: .number,
        title: .title,
        url: .html_url,
        comments: (.comments // 0),
        reactions_total: (.reactions.total_count // 0),
        updated_at: .updated_at,
        labels_md: (label_list),
        body: (body_preview)
      })
    | .[]
    | "### [\(.title)](\(.url)) (#\(.number))\n"
      + "- Reactions: **\(.reactions_total)** | Comments: **\(.comments)** | Updated: \(.updated_at)\n"
      + (if (.labels_md|length) > 0 then ("- Labels: " + .labels_md + "\n") else "" end)
      + "\n"
      + .body + "\n\n---\n"
  ' "$json_path" >> "$OUT_DIR/${slug}.md"
  rm -f "$headers_path"
}

# Focus: agent / coding-agent 类产品（来自 docs/opensource/README.md 的同类项目）
fetch_repo "openai/codex" "OpenAI Codex"
sleep "$SLEEP_SECONDS"
fetch_repo "shareAI-lab/Kode-cli" "Kode CLI"
sleep "$SLEEP_SECONDS"
fetch_repo "anomalyco/opencode" "OpenCode"
sleep "$SLEEP_SECONDS"
fetch_repo "QwenLM/qwen-code" "Qwen Code"

if [[ "$FOCUS_ONLY" != "1" || "$EXTRA" == "1" ]]; then
  sleep "$SLEEP_SECONDS"
  fetch_repo "code-yeongyu/oh-my-opencode" "Oh My OpenCode"
  sleep "$SLEEP_SECONDS"
  fetch_repo "continuedev/continue" "Continue"
  sleep "$SLEEP_SECONDS"
  fetch_repo "Aider-AI/aider" "Aider"
  sleep "$SLEEP_SECONDS"
  fetch_repo "OpenHands/OpenHands" "OpenHands"
  sleep "$SLEEP_SECONDS"
  fetch_repo "microsoft/autogen" "Microsoft AutoGen"
fi

echo "Done. Output: $OUT_DIR"
