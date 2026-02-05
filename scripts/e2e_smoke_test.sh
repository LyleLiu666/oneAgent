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
WS_DIR="${HOME_DIR}/ws"
mkdir -p "${WS_DIR}"
cleanup() {
  if [[ -n "${SERVER_PID:-}" ]]; then
    kill "${SERVER_PID}" >/dev/null 2>&1 || true
    wait "${SERVER_PID}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${MOCK_LLM_PID:-}" ]]; then
    kill "${MOCK_LLM_PID}" >/dev/null 2>&1 || true
    wait "${MOCK_LLM_PID}" >/dev/null 2>&1 || true
  fi
  rm -rf "${HOME_DIR}"
}
trap cleanup EXIT

echo "[e2e] start server: port=${PORT}"
ONEAGENT_HOME="${HOME_DIR}" \
  ONEAGENT_DISABLE_DAILY_LEARNING=1 \
  ONEAGENT_PANDOC_CMD=pandoc-does-not-exist \
  "${ROOT_DIR}/dist/oneagent" serve --auth-mode none --bind 127.0.0.1 --port "${PORT}" --workspace "${WS_DIR}" >/tmp/oneagent_e2e.log 2>&1 &
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
PORT="${PORT}" WS_DIR="${WS_DIR}" python3 - <<'PY'
import json
import os
import urllib.request
import urllib.error

port = os.environ["PORT"]
ws_dir = os.environ["WS_DIR"]

with urllib.request.urlopen(f"http://127.0.0.1:{port}/api/config") as r:
    data = json.loads(r.read().decode("utf-8"))

expected_ws = os.path.realpath(ws_dir)
got_ws = os.path.realpath(str(data.get("default_workspace", "")))
assert got_ws == expected_ws, data
assert data.get("base_url") == f"http://localhost:{port}", data
print("[e2e] /api/config OK")

# Document export should fail with actionable message when pandoc is missing.
report_md = os.path.join(ws_dir, "report.md")
with open(report_md, "w", encoding="utf-8") as f:
    f.write("# hello\n")

req = urllib.request.Request(
    f"http://127.0.0.1:{port}/api/documents/export",
    method="POST",
    headers={"Content-Type": "application/json"},
    data=json.dumps(
        {
            "workspace": ws_dir,
            "input_path": "report.md",
            "format": "docx",
        }
    ).encode("utf-8"),
)
try:
    with urllib.request.urlopen(req) as r:
        raise AssertionError(f"expected error, got status={r.status}")
except urllib.error.HTTPError as e:
    payload = json.loads(e.read().decode("utf-8"))
    msg = str(payload.get("error", "")).lower()
    assert "pandoc" in msg and "doctor" in msg, payload
    print("[e2e] POST /api/documents/export missing pandoc OK")

# Work ledger digest should always be readable.
with urllib.request.urlopen(f"http://127.0.0.1:{port}/api/ledger/digests/today") as r:
    d = json.loads(r.read().decode("utf-8"))
assert "day_key" in d, d
assert "markdown" in d, d
print("[e2e] /api/ledger/digests/today OK")

# Daily learning job endpoint should be callable (job may or may not exist).
try:
    with urllib.request.urlopen(f"http://127.0.0.1:{port}/api/ledger/learning/jobs/today") as r:
        job = json.loads(r.read().decode("utf-8"))
    assert isinstance(job, dict), job
    print("[e2e] /api/ledger/learning/jobs/today OK")
except Exception:
    print("[e2e] /api/ledger/learning/jobs/today (not found) OK")

# Ledger status summary should be callable.
with urllib.request.urlopen(f"http://127.0.0.1:{port}/api/ledger/status/today") as r:
    st = json.loads(r.read().decode("utf-8"))
assert "day_key" in st, st
assert "digest_exists" in st, st
assert "learning_job_status" in st, st
assert "sop_proposed_count" in st, st
print("[e2e] /api/ledger/status/today OK")

# Create a SOP suggestion (even without real receipt evidence in v1).
req = urllib.request.Request(
    f"http://127.0.0.1:{port}/api/ledger/sop_suggestions",
    method="POST",
    headers={"Content-Type": "application/json"},
    data=json.dumps(
        {
            "title": "SOP: e2e smoke",
            "draft_skill": "# SOP\n\n1. step\n",
            "evidence_receipt_ids": ["r1", "r2"],
        }
    ).encode("utf-8"),
)
with urllib.request.urlopen(req) as r:
    created = json.loads(r.read().decode("utf-8"))
assert created.get("suggestion_id"), created
assert created.get("status") == "proposed", created
print("[e2e] POST /api/ledger/sop_suggestions OK")

with urllib.request.urlopen(f"http://127.0.0.1:{port}/api/ledger/sop_suggestions?status=proposed") as r:
    lst = json.loads(r.read().decode("utf-8"))
assert isinstance(lst, list) and len(lst) >= 1, lst
print("[e2e] GET /api/ledger/sop_suggestions OK")

# Generator should be callable even if it returns empty.
req = urllib.request.Request(
    f"http://127.0.0.1:{port}/api/ledger/sop_suggestions/generate",
    method="POST",
    headers={"Content-Type": "application/json"},
    data=json.dumps({"count": 1, "lookback_days": 7}).encode("utf-8"),
)
with urllib.request.urlopen(req) as r:
    gen = json.loads(r.read().decode("utf-8"))
assert isinstance(gen, list), gen
print("[e2e] POST /api/ledger/sop_suggestions/generate OK")
PY
curl -sSf "http://127.0.0.1:${PORT}/" | head -n 2 >/dev/null

echo "[e2e] start mock llm (for secretary triage)"
MOCK_LLM_PORT_FILE="${HOME_DIR}/mock_llm_port"
MOCK_LLM_LOG="${HOME_DIR}/mock_llm.log"
MOCK_LLM_PORT_FILE="${MOCK_LLM_PORT_FILE}" python3 - <<'PY' >"${MOCK_LLM_LOG}" 2>&1 &
import json
import os
import re
from http.server import BaseHTTPRequestHandler, HTTPServer

port_file = os.environ["MOCK_LLM_PORT_FILE"]

class Handler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        return

    def do_POST(self):
        if self.path != "/chat/completions":
            self.send_response(404)
            self.end_headers()
            return

        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length)
        try:
            req = json.loads(body.decode("utf-8"))
        except Exception:
            self.send_response(400)
            self.end_headers()
            return

        messages = req.get("messages") or []
        system = ""
        combined = []
        for m in messages:
            if (m.get("role") == "system") and not system:
                system = str(m.get("content") or "")
            combined.append(str(m.get("content") or ""))
        combined_text = "\n".join(combined)

        if "ONEAGENT_SECRETARY_ACK" in system:
            self.send_response(400)
            self.send_header("Content-Type", "text/plain")
            self.end_headers()
            self.wfile.write(b"unexpected ack request (quick-ack is disabled)")
            return

        if "ONEAGENT_SECRETARY_TRIAGE" not in system:
            self.send_response(400)
            self.send_header("Content-Type", "text/plain")
            self.end_headers()
            self.wfile.write(b"unexpected system prompt")
            return

        # Use bulk cancel (no task_id) so we don't depend on short IDs being unique.
        task_actions = [{"action": "cancel"}]
        summary = "我已经把当前这条线上的任务都关掉了（相当于取消，不删证据）。"

        args = {
            "intent": "dispatch",
            "summary_message": summary,
            "tasks": [],
            "task_actions": task_actions,
            "questions": [],
        }

        resp = {
            "id": "cmpl-test",
            "choices": [
                {
                    "message": {
                        "role": "assistant",
                        "content": "",
                        "tool_calls": [
                            {
                                "id": "call_1",
                                "type": "function",
                                "function": {
                                    "name": "secretary_triage_plan",
                                    "arguments": json.dumps(args, ensure_ascii=False),
                                },
                            }
                        ],
                    },
                    "finish_reason": "stop",
                }
            ],
        }

        payload = json.dumps(resp, ensure_ascii=False).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

httpd = HTTPServer(("127.0.0.1", 0), Handler)
with open(port_file, "w", encoding="utf-8") as f:
    f.write(str(httpd.server_port))
httpd.serve_forever()
PY
MOCK_LLM_PID=$!

for _ in $(seq 1 50); do
  if [[ -s "${MOCK_LLM_PORT_FILE}" ]]; then
    break
  fi
  sleep 0.1
done
MOCK_LLM_PORT="$(cat "${MOCK_LLM_PORT_FILE}")"

PORT="${PORT}" MOCK_LLM_PORT="${MOCK_LLM_PORT}" python3 - <<'PY'
import json
import os
import urllib.request

port = os.environ["PORT"]
mock_port = os.environ["MOCK_LLM_PORT"]
base = f"http://127.0.0.1:{port}"

def post(path, obj):
    req = urllib.request.Request(
        base + path,
        method="POST",
        headers={"Content-Type": "application/json"},
        data=json.dumps(obj).encode("utf-8"),
    )
    with urllib.request.urlopen(req) as r:
        return json.loads(r.read().decode("utf-8"))

provider = post(
    "/api/llm/providers",
    {
        "name": "mock-e2e",
        "provider_type": "openai",
        "base_url": f"http://127.0.0.1:{mock_port}",
        "api_key": "sk-test",
    },
)
post(
    "/api/llm/models",
    {
        "provider_id": provider.get("id"),
        "name": "mock-model",
        "model": "gpt-test",
        "is_default": True,
    },
)
print("[e2e] secretary LLM configured")
PY

echo "[e2e] ui smoke (playwright)"
cd "${ROOT_DIR}/frontend"

if [[ "$(uname -s)" == "Linux" ]]; then
  npx playwright install --with-deps chromium >/dev/null
else
  npx playwright install chromium >/dev/null
fi

PLAYWRIGHT_BASE_URL="http://127.0.0.1:${PORT}" \
  ONEAGENT_E2E_WORKSPACE="${WS_DIR}" \
  npm run test:e2e -- --project=chromium --grep smoke

echo "[e2e] OK"
