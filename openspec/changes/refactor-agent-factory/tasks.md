## 1. Spec Updates (this PR)
- [x] 1.1 Add new capability spec delta: `system-agent-factory`
- [x] 1.2 Update `system-prompt-assembly` to require shared factory/assembler usage (stable vs volatile)
- [x] 1.3 Update `system-toolcalling-reliability` to cover structured outputs without forcing plain-text JSON
- [x] 1.4 Update `system-secretary-orchestration` to consume Agent Factory (best-effort) and remove brittle keyword routing
- [x] 1.5 Run `openspec change validate refactor-agent-factory --strict --no-interactive`

## 2. Implementation (after approval)
- [x] 2.1 Introduce `backend/internal/agent/**` (AgentSpec + AgentFactory + AgentRuntime)
- [x] 2.2 Add prompt stability tests (stable prefix / volatile context / cache key)
- [x] 2.3 Add structured output channel (tool-call first, fallback to loose tags)
- [x] 2.4 Migrate Secretary SW planner to AgentFactory (no more “strict JSON in text”)
- [ ] 2.5 Migrate Secretary SU report generation to AgentFactory (optional; keep message list append-only)
- [ ] 2.6 Keep Worker Chat behavior stable; migrate only if tests prove zero regression

## 3. Validation
- [x] 3.1 Backend: `cd backend && go test ./... -count=1`
- [x] 3.2 Frontend: `cd frontend && npm test -- --run`
- [ ] 3.3 Manual smoke (local):
  - [ ] /secretary: 连续消息 → triage → 有明确下一步
  - [ ] 进度询问（“任务完成得怎么样/有几个任务在进行”）不会再反问 workspace/任务是哪一个
  - [ ] trace 可定位：tool 协议/kv-cache/关键日志指针可见
