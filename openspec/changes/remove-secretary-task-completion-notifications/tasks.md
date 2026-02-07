## 1. Spec
- [x] 1.1 Add `chat-ux` delta requirement: task completion MUST NOT inject chat messages
- [x] 1.2 Run `openspec validate remove-secretary-task-completion-notifications --strict --no-interactive`

## 2. Frontend
- [x] 2.1 Remove `task-completed` chat injection handlers in ChatBox/SecretaryChatBox
- [x] 2.2 Filter legacy completion notification templates from secretary local messages

## 3. Tests
- [x] 3.1 Update ChatBox/SecretaryChatBox tests
- [x] 3.2 Add secretaryLocalMessages filtering regression test

## 4. Validation
- [x] 4.1 `npm --prefix frontend test -- --run`

