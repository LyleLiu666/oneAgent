#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

DATASET="${BENCHMARK_DATASET:-$ROOT/benchmarks/datasets/head2head_mvp.json}"
LIMIT="${BENCHMARK_LIMIT:-0}"
OUT_DIR="${BENCHMARK_OUT_DIR:-}"
BASELINE="${BENCHMARK_BASELINE:-}"

ARGS=(--repo-root "$ROOT" --dataset "$DATASET")
if [[ -n "${OUT_DIR}" ]]; then
  ARGS+=(--out "$OUT_DIR")
fi
if [[ -n "${BASELINE}" ]]; then
  ARGS+=(--baseline "$BASELINE")
fi
if [[ "${LIMIT}" != "0" ]]; then
  ARGS+=(--limit "$LIMIT")
fi

cd "$ROOT/backend"
go run ./cmd/benchmark "${ARGS[@]}"

