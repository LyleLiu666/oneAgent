#!/usr/bin/env bash

# 一键构建并启动（默认 PROFILE=dev，ONEAGENT_HOME 默认用当前 repo 根目录）：
# start_local.sh
# 想按 LAN 模式启动（监听 0.0.0.0）：
# start_local.sh
# 登录 token 文件在：
# $ONEAGENT_HOME/.oneagent/config/auth_token（可 cat 出来粘贴到登录页）
# 只跑自检不启动：
# start_local.sh --doctor-only

set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT_DIR/dist/oneagent"

# Defaults (override via env)
: "${ONEAGENT_HOME:="$ROOT_DIR"}"      # project-local state: $ONEAGENT_HOME/.oneagent/...
: "${PROFILE:=dev}"                    # dev=127.0.0.1 by default; local=0.0.0.0 by default
: "${AUTH_MODE:=token}"                # token|none
: "${PORT:=8080}"
: "${BIND:=}"                          # empty -> use profile default
: "${ENABLE_TRACE:=false}"             # true|false
: "${LOG_RETENTION_DAYS:=30}"

usage() {
  cat <<'EOF'
Start oneAgent in local-tool mode (no Docker).

Usage:
  ./scripts/start_local.sh [--no-build] [--doctor-only] [--build-only]

Environment (optional):
  ONEAGENT_HOME         Internal state root (default: repo root; state lives under $ONEAGENT_HOME/.oneagent/)
  PROFILE               local|dev (default: dev)
  AUTH_MODE             token|none (default: token)
  BIND                  bind address (default: based on profile)
  PORT                  port (default: 8080)
  ENABLE_TRACE          true|false (default: false)
  LOG_RETENTION_DAYS    days (default: 30)

Notes:
  - Login token file: $ONEAGENT_HOME/.oneagent/config/auth_token
  - Workspace is configured per-session in the UI (Chat top bar).
EOF
}

build() {
  echo "[oneagent] build (frontend + backend)..."
  (cd "$ROOT_DIR" && make build)
}

doctor() {
  echo "[oneagent] doctor..."
  BIND="$BIND" PORT="$PORT" "$BIN" doctor \
    --home "$ONEAGENT_HOME" \
    --profile "$PROFILE" \
    --auth-mode "$AUTH_MODE" \
    --enable-trace "$ENABLE_TRACE" \
    --log-retention-days "$LOG_RETENTION_DAYS"
}

serve() {
  local -a args
  args=(serve
    --home "$ONEAGENT_HOME"
    --profile "$PROFILE"
    --port "$PORT"
    --auth-mode "$AUTH_MODE"
    --enable-trace "$ENABLE_TRACE"
    --log-retention-days "$LOG_RETENTION_DAYS"
  )
  if [[ -n "${BIND}" ]]; then
    args+=(--bind "$BIND")
  fi

  echo "[oneagent] serve..."
  echo "[oneagent] open: http://localhost:${PORT}"
  echo "[oneagent] token file: ${ONEAGENT_HOME}/.oneagent/config/auth_token"
  exec "$BIN" "${args[@]}"
}

main() {
  if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
    usage
    return 0
  fi

  local do_build=true
  local do_doctor=true
  local do_serve=true

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --no-build)
        do_build=false
        shift
        ;;
      --doctor-only)
        do_build=false
        do_serve=false
        shift
        ;;
      --build-only)
        do_doctor=false
        do_serve=false
        shift
        ;;
      --help|-h)
        usage
        return 0
        ;;
      *)
        echo "unknown arg: $1" >&2
        usage >&2
        return 2
        ;;
    esac
  done

  if $do_build; then
    build
  fi

  if [[ ! -f "$BIN" ]]; then
    echo "missing binary: $BIN (run without --no-build, or run: make build)" >&2
    return 1
  fi

  if $do_doctor; then
    doctor
  fi

  if $do_serve; then
    serve
  fi
}

main "$@"

