## 1. Implementation
- [x] 1.1 Add spec delta for opt-in live LLM regression suite
- [x] 1.2 Backend E2E: add live regression runner (skipped unless env enabled)
- [x] 1.3 Report: write JSON + Markdown summary to `.oneagent/tmp/`
- [x] 1.4 Add cases: JSON vs XML + long-text (>=3000 runes) tool arguments
- [x] 1.5 Run live suite locally and keep report path for follow-up UX fixes (latest: `.oneagent/tmp/live_llm_regression_20260130_102608.json`)
- [x] 1.6 Run `openspec validate add-live-llm-regression-suite --strict --no-interactive`
- [x] 1.7 Run `cd backend && go test ./...`
