#!/bin/bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "This installer is for macOS (launchd) only." >&2
  exit 1
fi

REPO_ROOT="${1:-}"
if [[ -z "$REPO_ROOT" ]]; then
  REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
fi

TEMPLATE="$REPO_ROOT/docs/opensource/agent_issues/scheduler/com.oneagent.fetch_agent_issues.plist.template"
if [[ ! -f "$TEMPLATE" ]]; then
  echo "Template not found: $TEMPLATE" >&2
  exit 1
fi

DEST_DIR="$HOME/Library/LaunchAgents"
DEST_PLIST="$DEST_DIR/com.oneagent.fetch_agent_issues.plist"

mkdir -p "$DEST_DIR"
mkdir -p "$REPO_ROOT/.oneagent/tmp"

sed "s|__REPO_ROOT__|$REPO_ROOT|g" "$TEMPLATE" > "$DEST_PLIST"

# Reload service (best-effort, compatible across macOS versions).
launchctl bootout "gui/${UID}" "$DEST_PLIST" >/dev/null 2>&1 || true
launchctl bootstrap "gui/${UID}" "$DEST_PLIST" >/dev/null 2>&1 || launchctl load -w "$DEST_PLIST" >/dev/null 2>&1

echo "Installed launchd job: com.oneagent.fetch_agent_issues"
echo "Plist: $DEST_PLIST"
echo "Logs:  $REPO_ROOT/.oneagent/tmp/fetch_agent_issues.(out|err)"
echo "Run now: launchctl kickstart -k \"gui/${UID}/com.oneagent.fetch_agent_issues\""

