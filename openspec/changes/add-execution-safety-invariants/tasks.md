## 1. Implementation
- [ ] 1.1 Add spec deltas for approval + rollback + non-bypassable script execution
- [ ] 1.2 Backend: tool registry adds `mutating` + `reversibility` metadata (rollbackable/irreversible) (or equivalent)
- [ ] 1.3 Backend: approval pipeline (deny/approve) + evidence persisted into trace/receipt (single-user)
- [ ] 1.4 Backend: attempt-level rollback boundary (worktree preferred; fallback to checkpoint/restore)
- [ ] 1.5 Backend: command runner enforces “no hard boundary → readonly only”; hard boundary required for write-capable command profiles
- [ ] 1.6 Backend tests: approval gating + rollback restore + audit evidence
- [ ] 1.7 Frontend: minimal approval UI + status display (and “why denied”)
- [ ] 1.8 Run `openspec validate add-execution-safety-invariants --strict --no-interactive` and keep tests green

