# Benchmarks

This folder contains reproducible benchmark datasets and runner docs for measuring end-to-end delivery quality.

## Head-to-Head (MVP)

- Dataset: `benchmarks/datasets/head2head_mvp.json`
- Runner output (default): `.oneagent/tmp/benchmarks/<run_id>/`

### Goals

Track delivery-quality metrics over time (and compare runs best-effort):

- first pass success rate
- recovery success rate
- human intervention count
- evidence completeness
- cost per successful delivery (best-effort)

### How To Run (Mock / Deterministic)

From repo root:

```bash
bash scripts/benchmark_run.sh
```

Smoke (>=3 cases):

```bash
BENCHMARK_LIMIT=3 bash scripts/benchmark_run.sh
```

### Dataset Structure (Simplified)

Each benchmark case is:

- `prompt`: what the agent is asked to deliver
- `workspace.seed_files`: deterministic starting workspace state
- `acceptance`: objective checks (files + artifacts)
- `evidence_required`: which artifacts MUST exist for “evidence completeness”
- `mock.attempts[*].responses`: scripted provider replies for deterministic execution

The runner treats each case as a task+acceptance+evidence unit and produces:

- `report.json`: machine-readable results
- `report.md`: human summary (with best-effort baseline diff)

