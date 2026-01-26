#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "[e2e] build"
cd "${ROOT_DIR}"
make build >/dev/null

PORT="$(python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
)"

HOME_DIR="$(mktemp -d)"
cleanup() {
  if [[ -n "${SERVER_PID:-}" ]]; then
    kill "${SERVER_PID}" >/dev/null 2>&1 || true
    wait "${SERVER_PID}" >/dev/null 2>&1 || true
  fi
  rm -rf "${HOME_DIR}"
}
trap cleanup EXIT

echo "[e2e] start server: port=${PORT}"
ONEAGENT_HOME="${HOME_DIR}" \
  "${ROOT_DIR}/dist/oneagent" serve --auth-mode none --bind 127.0.0.1 --port "${PORT}" >/tmp/oneagent_e2e.log 2>&1 &
SERVER_PID=$!

echo "[e2e] wait for /health"
for _ in $(seq 1 50); do
  if curl -sSf "http://127.0.0.1:${PORT}/health" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done

curl -sSf "http://127.0.0.1:${PORT}/health" >/dev/null
curl -sSf "http://127.0.0.1:${PORT}/api/tools" >/dev/null
curl -sSf "http://127.0.0.1:${PORT}/" | head -n 2 >/dev/null

echo "[e2e] OK"

