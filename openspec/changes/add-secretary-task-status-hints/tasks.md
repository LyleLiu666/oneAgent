## 1. Spec & Design (planning first)
- [x] 1.1 Define task status hint requirement in `chat-ux` delta

## 2. Implementation (after proposal approval)
- [ ] 2.1 Add reusable composable for task list polling + active count
- [ ] 2.2 Extend `SecretaryStatusHints` to show active task count in secretary mode

## 3. Tests & Validation
- [ ] 3.1 Unit tests: task hint shows/hides and click-to-full navigates to `/tasks`
- [ ] 3.2 Regression: ChatBox tests keep passing (mock task api)
- [ ] 3.3 `npm test -- --run` (frontend)
- [ ] 3.4 `openspec validate add-secretary-task-status-hints --strict --no-interactive`

