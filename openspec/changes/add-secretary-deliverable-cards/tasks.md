## 1. Spec & Design (planning first)
- [ ] 1.1 Define secretary deliverable cards requirement in `chat-ux` delta (task artifacts only)

## 2. Implementation (after proposal approval)
- [ ] 2.1 Add `SecretaryTaskDeliverables` component to render completed task artifact cards (low-noise)
- [ ] 2.2 Add artifact preview modal (best-effort) using task artifact endpoints
- [ ] 2.3 Mount deliverable cards in secretary chat without bloating `ChatBox.vue`

## 3. Tests & Validation
- [ ] 3.1 Unit tests: renders cards from completed tasks + opens artifact modal
- [ ] 3.2 `npm test -- --run` (frontend)
- [ ] 3.3 `openspec validate add-secretary-deliverable-cards --strict --no-interactive`

