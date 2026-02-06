#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "[ci] openspec truth check"
bash "${ROOT_DIR}/scripts/openspec_truth_check.sh"

echo "[ci] backend tests"
cd "${ROOT_DIR}/backend"
go test ./...

echo "[ci] release script self-test"
cd "${ROOT_DIR}"
bash scripts/release_local_selftest.sh

echo "[ci] windows cross-build"
cd "${ROOT_DIR}/backend"
env GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...
