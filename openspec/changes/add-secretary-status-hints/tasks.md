## 1. Spec & Design (planning first)
- [x] 1.1 Define secretary-mode status hints requirement in `chat-ux` delta
- [ ] 1.2 Decide the minimal hint set for v1 (SOP only vs SOP+task running)

## 2. Implementation (after proposal approval)
- [ ] 2.1 Add reusable composable for `GET /api/ledger/status/today` polling
- [ ] 2.2 Refactor Sidebar SOP badge to reuse the composable
- [ ] 2.3 Add `SecretaryStatusHints` component (low-noise badge + click-to-full)
- [ ] 2.4 Mount hints in secretary mode chat header without bloating `ChatBox.vue`

## 3. Tests & Validation
- [ ] 3.1 Unit test: secretary hints renders SOP badge and navigates to governance
- [ ] 3.2 Regression: Sidebar badge still works
- [ ] 3.3 `npm test -- --run` (frontend)
- [ ] 3.4 `openspec validate add-secretary-status-hints --strict --no-interactive`

