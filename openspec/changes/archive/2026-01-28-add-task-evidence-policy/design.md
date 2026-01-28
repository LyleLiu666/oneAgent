# Design: Evidence policy

## Evidence artifacts
- `findings_path` (already)
- `trace_log_path` (already)
- `test_report_path` (new; optional but encouraged when tests exist)

## Generation strategy (v1)
- Best-effort heuristics per language:
  - Go: `go test ./...` → capture stdout/stderr to report file
  - Node: `npm test -- --run` (if detected) → report file
  - Python: `pytest -q` (if detected) → report file
- Provide config/flags to disable auto-test for sensitive repos

## Observer contract
- Observer stays read-only; it can only read `test_report_path` and other files.
- If no report is available, observer should downgrade confidence and ask user for confirmation or next action.

