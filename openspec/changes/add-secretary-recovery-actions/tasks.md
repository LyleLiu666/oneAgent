## 1. Spec & Design (planning first)
- [ ] 1.1 Define secretary recovery actions requirement in `chat-ux` delta (task failures)

## 2. Implementation (after proposal approval)
- [ ] 2.1 Surface low-noise “needs attention” cards for failed terminal tasks in secretary mode
- [ ] 2.2 Provide one-click resume action (best-effort)
- [ ] 2.3 Provide one-click full-mode troubleshooting entry (best-effort)

## 3. Tests & Validation
- [ ] 3.1 Unit tests: failed task is surfaced + resume triggers API + troubleshoot navigates
- [ ] 3.2 `npm test -- --run` (frontend)
- [ ] 3.3 `openspec validate add-secretary-recovery-actions --strict --no-interactive`

