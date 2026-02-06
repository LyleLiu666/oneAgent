#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "[openspec] truth check: start"

fail=0

echo "[openspec] check: spec Purpose placeholders"
if command -v rg >/dev/null 2>&1; then
  purpose_matches="$(rg -n '^TBD - created by archiving change' "${ROOT_DIR}/openspec/specs" || true)"
else
  purpose_matches="$(grep -R -n '^TBD - created by archiving change' "${ROOT_DIR}/openspec/specs" || true)"
fi

if [[ -n "${purpose_matches}" ]]; then
  echo "[openspec] ERROR: placeholder Purpose found in capability specs:"
  echo "${purpose_matches}"
  fail=1
fi

echo "[openspec] check: active changes set (from filesystem)"
active_changes=()
for d in "${ROOT_DIR}/openspec/changes"/*; do
  [[ -d "${d}" ]] || continue
  id="$(basename "${d}")"
  [[ "${id}" == "archive" ]] && continue
  active_changes+=("${id}")
done

sorted_active_changes="$({
  if [[ ${#active_changes[@]} -gt 0 ]]; then
    printf '%s\n' "${active_changes[@]}" | LC_ALL=C sort
  fi
} | sed '/^$/d')"

echo "[openspec] check: review snapshot matches active changes (best-effort)"
review_md="${ROOT_DIR}/openspec/changes/review.md"
snapshot_ids=()
if [[ -f "${review_md}" ]]; then
  in_snapshot=0
  while IFS= read -r line; do
    if [[ "${line}" == "## Snapshot（来自 \`openspec list\`）" ]]; then
      in_snapshot=1
      continue
    fi
    if [[ ${in_snapshot} -eq 1 ]]; then
      # End snapshot block at the next top-level header.
      if [[ "${line}" == "## "* ]]; then
        break
      fi
      if [[ "${line}" =~ ^-\ \`([^\`]*)\` ]]; then
        snapshot_ids+=("${BASH_REMATCH[1]}")
      fi
    fi
  done < "${review_md}"
fi

sorted_snapshot_ids="$({
  if [[ ${#snapshot_ids[@]} -gt 0 ]]; then
    printf '%s\n' "${snapshot_ids[@]}" | LC_ALL=C sort
  fi
} | sed '/^$/d')"

if [[ -n "${sorted_snapshot_ids}" ]]; then
  if [[ "${sorted_snapshot_ids}" != "${sorted_active_changes}" ]]; then
    echo "[openspec] ERROR: openspec/changes/review.md snapshot is out of date"
    echo "[openspec] expected (active changes):"
    echo "${sorted_active_changes}"
    echo "[openspec] got (snapshot):"
    echo "${sorted_snapshot_ids}"
    fail=1
  fi
else
  # Keep best-effort to avoid blocking when snapshot is intentionally removed.
  if [[ -n "${sorted_active_changes}" ]]; then
    echo "[openspec] WARNING: review.md snapshot block is empty; consider updating it"
  fi
fi

echo "[openspec] check: roadmap does not claim 'no active changes' when active changes exist"
roadmap_longterm="${ROOT_DIR}/openspec/roadmap-longterm.md"
if [[ -f "${roadmap_longterm}" ]] && [[ -n "${sorted_active_changes}" ]]; then
  if command -v rg >/dev/null 2>&1; then
    if rg -q '当前 active changes 均已' "${roadmap_longterm}"; then
      echo "[openspec] ERROR: roadmap-longterm.md still claims no active changes, but active changes exist"
      echo "[openspec] active changes:"
      echo "${sorted_active_changes}"
      fail=1
    fi
  else
    if grep -q '当前 active changes 均已' "${roadmap_longterm}"; then
      echo "[openspec] ERROR: roadmap-longterm.md still claims no active changes, but active changes exist"
      echo "[openspec] active changes:"
      echo "${sorted_active_changes}"
      fail=1
    fi
  fi
fi

if [[ "${fail}" == "0" ]]; then
  echo "[openspec] truth check: OK"
else
  echo "[openspec] truth check: FAILED"
  exit 1
fi
