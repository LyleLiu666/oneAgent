#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

TMP_DIR="$(mktemp -d)"
cleanup() { rm -rf "${TMP_DIR}"; }
trap cleanup EXIT

PIN_FILE="${TMP_DIR}/portable_git_url.txt"
echo "# pinned portable git url" >"${PIN_FILE}"
echo "https://example.com/PortableGit-FAKE-64-bit.7z.exe" >>"${PIN_FILE}"

export ONEAGENT_RELEASE_LIB=1
export PORTABLE_GIT_URL=""
export PORTABLE_GIT_URL_FILE="${PIN_FILE}"

# shellcheck disable=SC1091
source "${ROOT_DIR}/scripts/release_local.sh"

got="$(resolve_portable_git_url)"
if [[ "${got}" != "https://example.com/PortableGit-FAKE-64-bit.7z.exe" ]]; then
  echo "expected pinned url, got: ${got}" >&2
  exit 1
fi

export PORTABLE_GIT_URL="https://env.example/PortableGit-ENV-64-bit.7z.exe"
got="$(resolve_portable_git_url)"
if [[ "${got}" != "https://env.example/PortableGit-ENV-64-bit.7z.exe" ]]; then
  echo "expected env url to win, got: ${got}" >&2
  exit 1
fi

echo "[release_selftest] OK"

