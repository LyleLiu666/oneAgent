## 1. Implementation
- [x] 1.1 Add spec deltas for approval + rollback + non-bypassable script execution
- [x] 1.2 Backend: tool registry adds `mutating` + `reversibility` metadata (rollbackable/irreversible) (or equivalent)
- [x] 1.3 Backend: approval pipeline (deny/approve) + evidence persisted into trace/receipt (single-user)
- [x] 1.4 Backend: attempt-level rollback boundary (worktree preferred; fallback to checkpoint/restore)
- [x] 1.5 Backend: command runner enforces “no hard boundary → readonly only”; hard boundary required for write-capable command profiles
- [x] 1.6 Backend tests: approval gating + rollback restore + audit evidence
- [x] 1.7 Frontend: minimal approval UI + status display (and “why denied”)
- [x] 1.8 Run `openspec validate add-execution-safety-invariants --strict --no-interactive` and keep tests green
