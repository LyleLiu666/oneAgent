## 1. Spec
- [ ] 1.1 Update `system-secretary-orchestration` delta requirements (SU fast-path, no SW sync)
- [ ] 1.2 Update `chat-ux` delta requirements (recovery panel-only, not persisted to chat messages)
- [ ] 1.3 Run `openspec validate update-secretary-su-fast-path --strict --no-interactive`

## 2. Backend
- [ ] 2.1 Refactor secretary triage to generate plan as SU (read-only tools, <=5 steps)
- [ ] 2.2 Inject SettingsDB into secretary tool loop context
- [ ] 2.3 Ensure SU direct answers do not create/enqueue tasks
- [ ] 2.4 Add/adjust unit tests for direct-answer fast path and settings injection

## 3. Frontend
- [ ] 3.1 Keep recovery prompt panel-only (no persisted chat injection)
- [ ] 3.2 Update copy to “仅面板展示，不写入 chat message list”
- [ ] 3.3 Update/extend vitest coverage for secretary recovery rendering

## 4. Validation
- [ ] 4.1 `go test ./...`
- [ ] 4.2 `pnpm -C frontend test`

