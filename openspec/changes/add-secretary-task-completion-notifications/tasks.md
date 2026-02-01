## 1. Spec & Design (planning first)
- [ ] 1.1 Define task completion notification requirement in `chat-ux` delta

## 2. Implementation (after proposal approval)
- [ ] 2.1 Detect task latest attempt status transitions in `SecretaryTaskDeliverables` (best-effort)
- [ ] 2.2 Emit `task-completed` event with minimal payload
- [ ] 2.3 Append an assistant notification message in ChatBox on `task-completed` (secretary mode)

## 3. Tests & Validation
- [ ] 3.1 Unit test: deliverables emits completion event on transition (no history replay)
- [ ] 3.2 Unit test: ChatBox appends assistant message when event received (stub child)
- [ ] 3.3 `npm test -- --run` (frontend)
- [ ] 3.4 `openspec validate add-secretary-task-completion-notifications --strict --no-interactive`

