## 1. Spec & Design (planning first)
- [x] 1.1 Define secretary recovery actions requirement in `chat-ux` delta (task failures)

## 2. Implementation (after proposal approval)
- [x] 2.1 Surface low-noise “needs attention” cards for failed terminal tasks in secretary mode
- [x] 2.2 Provide one-click resume action (best-effort)
- [x] 2.3 Provide one-click full-mode troubleshooting entry (best-effort)

## 3. Tests & Validation
- [x] 3.1 Unit tests: failed task is surfaced + resume triggers API + troubleshoot navigates
- [x] 3.2 `npm test -- --run` (frontend)
- [x] 3.3 `openspec validate add-secretary-recovery-actions --strict --no-interactive`
