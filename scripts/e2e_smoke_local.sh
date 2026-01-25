#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

BIND="${BIND:-127.0.0.1}"
PORT="${PORT:-18080}"
ONEAGENT_HOME="${ONEAGENT_HOME:-}"
WORKSPACE="${WORKSPACE:-$ROOT_DIR}"

if [[ -z "${ONEAGENT_HOME}" ]]; then
  ONEAGENT_HOME="$(mktemp -d)"
  export ONEAGENT_HOME
fi

echo "[smoke] oneagent_home=${ONEAGENT_HOME}"
echo "[smoke] bind=${BIND} port=${PORT}"
echo "[smoke] workspace=${WORKSPACE}"

cd "${ROOT_DIR}"
make build >/dev/null

cleanup() {
  if [[ -n "${PID:-}" ]]; then
    kill "${PID}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

./dist/oneagent serve --bind "${BIND}" --port "${PORT}" --home "${ONEAGENT_HOME}" >/dev/null 2>&1 &
PID=$!

echo "[smoke] waiting for /health..."
for i in $(seq 1 50); do
  if curl -sf "http://${BIND}:${PORT}/health" >/dev/null; then
    break
  fi
  sleep 0.1
done

TOKEN_FILE="${ONEAGENT_HOME}/.oneagent/config/auth_token"
if [[ ! -f "${TOKEN_FILE}" ]]; then
  echo "[smoke] token file missing: ${TOKEN_FILE}" >&2
  exit 1
fi
TOKEN="$(cat "${TOKEN_FILE}")"

echo "[smoke] /api/me"
curl -sf -H "Authorization: Bearer ${TOKEN}" "http://${BIND}:${PORT}/api/me" | cat
echo

echo "[smoke] /api/tools"
curl -sf -H "Authorization: Bearer ${TOKEN}" "http://${BIND}:${PORT}/api/tools" >/dev/null

echo "[smoke] /api/sessions"
curl -sf -H "Authorization: Bearer ${TOKEN}" "http://${BIND}:${PORT}/api/sessions" >/dev/null

echo "[smoke] OK"

