## 1. Implementation
- [x] 1.1 Add spec deltas for `coding` profile (docker-only) and safety constraints
- [x] 1.2 Backend: add `coding` profile allowlist (git/find/go/python/node/npm/...)
- [x] 1.3 Backend: enforce `coding` profile only when `sandbox_mode=docker` (fail-closed / actionable error)
- [x] 1.4 Backend tests: profile allowlist + sandbox_mode constraint cases
- [x] 1.5 Run `openspec validate add-command-profile-coding --strict --no-interactive`
- [x] 1.6 Run `cd backend && go test ./...`
