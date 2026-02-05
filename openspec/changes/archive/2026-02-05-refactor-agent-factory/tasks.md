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
- [x] 2.5 Migrate Secretary SU report generation to AgentFactory (optional; keep message list append-only)
- [x] 2.6 Keep Worker Chat behavior stable; migrate only if tests prove zero regression

## 3. Validation
- [x] 3.1 Backend: `cd backend && go test ./... -count=1`
- [x] 3.2 Frontend: `cd frontend && npm test -- --run`
- [x] 3.3 Smoke (best-effort, automated):
  - [x] /secretary: progress intent uses task snapshot and returns deterministic progress reply（见 `backend/internal/secretary/orchestrator_progress_reply_test.go`）
  - [x] LLM 不可用时回退到任务看板快照（同上）
  - [x] tool_call/tool_result 与 trace/log 指针仍按既有链路留痕（已由现有 taskqueue / tool loop 覆盖；best-effort）
