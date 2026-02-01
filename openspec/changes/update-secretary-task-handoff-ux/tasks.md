## 1. Spec & Design (planning first)
- [x] 1.1 Define secretary handoff suggestion + chat receipt requirement in `chat-ux` delta

## 2. Implementation (after proposal approval)
- [x] 2.1 Add conservative heuristic to suggest task handoff on Send (secretary mode only)
- [x] 2.2 Add modal UI (low-noise) with accept / send-as-chat actions
- [x] 2.3 Append chat receipt messages after successful handoff (best-effort)

## 3. Tests & Validation
- [x] 3.1 Unit tests: suggestion gate + accept handoff does not call streamChat
- [x] 3.2 Unit tests: successful handoff appends chat receipt messages
- [x] 3.3 `npm test -- --run` (frontend)
- [x] 3.4 `openspec validate update-secretary-task-handoff-ux --strict --no-interactive`
