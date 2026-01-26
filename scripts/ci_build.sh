#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "[ci] backend tests"
cd "${ROOT_DIR}/backend"
go test ./...

echo "[ci] windows cross-build"
env GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...

