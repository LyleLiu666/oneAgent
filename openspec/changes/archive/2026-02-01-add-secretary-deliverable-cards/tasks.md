## 1. Spec & Design (planning first)
- [x] 1.1 Define secretary deliverable cards requirement in `chat-ux` delta (task artifacts only)

## 2. Implementation (after proposal approval)
- [x] 2.1 Add `SecretaryTaskDeliverables` component to render completed task artifact cards (low-noise)
- [x] 2.2 Add artifact preview modal (best-effort) using task artifact endpoints
- [x] 2.3 Mount deliverable cards in secretary chat without bloating `ChatBox.vue`

## 3. Tests & Validation
- [x] 3.1 Unit tests: renders cards from completed tasks + opens artifact modal
- [x] 3.2 `npm test -- --run` (frontend)
- [x] 3.3 `openspec validate add-secretary-deliverable-cards --strict --no-interactive`
