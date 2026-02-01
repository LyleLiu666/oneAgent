## 1. Spec & Design (planning first)
- [x] 1.1 Define secretary-mode status hints requirement in `chat-ux` delta
- [x] 1.2 Decide the minimal hint set for v1 (SOP only)

## 2. Implementation (after proposal approval)
- [x] 2.1 Add reusable composable for `GET /api/ledger/status/today` polling
- [x] 2.2 Refactor Sidebar SOP badge to reuse the composable
- [x] 2.3 Add `SecretaryStatusHints` component (low-noise badge + click-to-full)
- [x] 2.4 Mount hints in secretary mode chat header without bloating `ChatBox.vue`

## 3. Tests & Validation
- [x] 3.1 Unit test: secretary hints renders SOP badge and navigates to governance
- [x] 3.2 Regression: Sidebar badge still works
- [x] 3.3 `npm test -- --run` (frontend)
- [x] 3.4 `openspec validate add-secretary-status-hints --strict --no-interactive`
