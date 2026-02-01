## 1. Spec & Design (planning first)
- [ ] 1.1 Define secretary handoff suggestion + chat receipt requirement in `chat-ux` delta

## 2. Implementation (after proposal approval)
- [ ] 2.1 Add conservative heuristic to suggest task handoff on Send (secretary mode only)
- [ ] 2.2 Add modal UI (low-noise) with accept / send-as-chat actions
- [ ] 2.3 Append chat receipt messages after successful handoff (best-effort)

## 3. Tests & Validation
- [ ] 3.1 Unit tests: suggestion gate + accept handoff does not call streamChat
- [ ] 3.2 Unit tests: successful handoff appends chat receipt messages
- [ ] 3.3 `npm test -- --run` (frontend)
- [ ] 3.4 `openspec validate update-secretary-task-handoff-ux --strict --no-interactive`

