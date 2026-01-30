#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_DIR="$ROOT_DIR/snapshots"

TOP_N="${TOP_N:-10}"

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
  q_encoded="$(printf '%s' "$q" | jq -sRr @uri)"
  url="https://api.github.com/search/issues?q=${q_encoded}&sort=reactions&order=desc&per_page=${TOP_N}"

  if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    curl -sS \
      -H "Accept: application/vnd.github+json" \
      -H "X-GitHub-Api-Version: 2022-11-28" \
      -H "Authorization: Bearer ${GITHUB_TOKEN}" \
      "$url" \
      > "$OUT_DIR/${slug}.json"
  else
    curl -sS \
      -H "Accept: application/vnd.github+json" \
      -H "X-GitHub-Api-Version: 2022-11-28" \
      "$url" \
      > "$OUT_DIR/${slug}.json"
  fi

  if ! jq -e '.items and (.items|type=="array")' "$OUT_DIR/${slug}.json" >/dev/null 2>&1; then
    msg="$(jq -r '.message // empty' "$OUT_DIR/${slug}.json" 2>/dev/null || true)"
    status="$(jq -r '.status // empty' "$OUT_DIR/${slug}.json" 2>/dev/null || true)"

    cat > "$OUT_DIR/${slug}.md" <<EOF
# ${name} — Top Issues Snapshot (open, sorted by reactions)

- Fetched at (UTC): ${fetched_at}
- Repo: https://github.com/${repo}
- Query: \`${q} sort:reactions-desc\`
- Top N: ${TOP_N}

## Fetch failed

- Status: ${status}
- Message: ${msg}

Raw response saved at: \`${slug}.json\`
EOF
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
  ' "$OUT_DIR/${slug}.json" >> "$OUT_DIR/${slug}.md"
}

# Focus: agent / coding-agent 类产品（来自 docs/opensource/README.md 的同类项目）
fetch_repo "openai/codex" "OpenAI Codex"
fetch_repo "anomalyco/opencode" "OpenCode"
fetch_repo "code-yeongyu/oh-my-opencode" "Oh My OpenCode"
fetch_repo "QwenLM/qwen-code" "Qwen Code"
fetch_repo "shareAI-lab/Kode-cli" "Kode CLI"
fetch_repo "continuedev/continue" "Continue"
fetch_repo "Aider-AI/aider" "Aider"
fetch_repo "OpenHands/OpenHands" "OpenHands"
fetch_repo "microsoft/autogen" "Microsoft AutoGen"

echo "Done. Output: $OUT_DIR"
