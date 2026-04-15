#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
go_mod_path="${GO_MOD_PATH:-$repo_root/backend/go.mod}"

extract_required_version() {
  local module_path="$1"
  awk -v module="$module_path" '$1 == module { print $2; exit }' "$go_mod_path"
}

require_exact_tag() {
  local name="$1"
  local module_path="$2"
  local submodule_path="$3"
  local expected_tag
  local actual_tag

  expected_tag="$(extract_required_version "$module_path")"
  if [[ -z "$expected_tag" ]]; then
    echo "[sdk-pin] missing required version for $name in backend/go.mod" >&2
    exit 1
  fi

  if [[ ! -d "$repo_root/$submodule_path" ]]; then
    echo "[sdk-pin] missing submodule directory: $submodule_path" >&2
    exit 1
  fi

  actual_tag="$(git -C "$repo_root/$submodule_path" describe --tags --exact-match HEAD 2>/dev/null || true)"
  if [[ -z "$actual_tag" ]]; then
    echo "[sdk-pin] $name is not pinned to an exact tag: $submodule_path" >&2
    exit 1
  fi

  if [[ "$actual_tag" != "$expected_tag" ]]; then
    echo "[sdk-pin] $name tag mismatch: go.mod requires $expected_tag but $submodule_path is at $actual_tag" >&2
    exit 1
  fi

  echo "[sdk-pin] $name pinned at $actual_tag"
}

require_exact_tag \
  "agentsdk" \
  "codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/agentsdk.git" \
  "third_party/agentsdk"

require_exact_tag \
  "memorySdk" \
  "codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/memorySdk.git" \
  "third_party/memorySdk"
