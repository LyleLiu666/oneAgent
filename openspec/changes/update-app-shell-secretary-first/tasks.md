## 1. Spec & Design (planning first)
- [x] 1.1 Define secretary-first app shell requirements in OpenSpec deltas
- [x] 1.2 Identify impacted existing specs (work-ledger-ux sidebar badge)
- [ ] 1.3 Confirm routing strategy for default entry (`/` redirect vs ChatBox default mode)
- [ ] 1.4 Confirm deep-link behavior in secretary mode (prompt vs redirect)

## 2. Implementation (after proposal approval)
- [ ] 2.1 Add global `ui_mode` store (Pinia) and persistence
- [ ] 2.2 Update `frontend/src/App.vue` to render `Sidebar` only in `ui_mode=full`
- [ ] 2.3 Ensure mobile sidebar menu button is also hidden in `ui_mode=secretary`
- [ ] 2.4 Update `ChatBox` to read/write global `ui_mode` (migrate old localStorage key)
- [ ] 2.5 Implement default entry routing strategy (if choosing `/` redirect)
- [ ] 2.6 Implement deep-link UX (if choosing prompt-in-secretary)

## 3. Tests & Validation
- [ ] 3.1 Frontend unit/component tests: sidebar hidden in secretary, visible in full (no text-coupled selectors)
- [ ] 3.2 E2E smoke: mode persists across reload and affects app shell
- [ ] 3.3 `openspec validate update-app-shell-secretary-first --strict --no-interactive`

