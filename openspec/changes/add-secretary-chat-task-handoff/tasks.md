## 1. Spec & Design (planning first)
- [x] 1.1 Define secretary task handoff requirement in `chat-ux` delta

## 2. Implementation (after proposal approval)
- [x] 2.1 Add secretary-only “handoff to task” action in chat composer
- [x] 2.2 Best-effort workspace resolution for task creation (no workspace → prompt/handle cancel)
- [x] 2.3 Low-noise success/error feedback without switching modes

## 3. Tests & Validation
- [x] 3.1 Unit tests: button visibility + click creates task + preserves input on failure/cancel
- [x] 3.2 `npm test -- --run` (frontend)
- [x] 3.3 `openspec validate add-secretary-chat-task-handoff --strict --no-interactive`
