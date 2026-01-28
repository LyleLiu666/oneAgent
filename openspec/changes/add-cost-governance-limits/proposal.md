# Change: Add cost/token governance limits for long-running tasks

## Why
目前 task queue 已有 steps/runtime 的默认 limits，但在“多 provider + 长任务 + 部门推广”的目标下，还需要成本维度的治理：
- provider 的 usage/cost 字段不一致，用户难以判断“跑了多少、花了多少”
- 缺少预算上限时，长任务可能在 loop 中产生不可控成本

## What Changes
- 为 Task limits 增加可选的 cost/token 预算字段（并可配置默认值与 cap）
- 统一记录 LLM usage 账本（per attempt 聚合），并在 UI/receipt 中可见
- 超限行为可解释：标记为终态并给出“预算耗尽”的原因与建议（例如 resume 时提高预算或拆分任务）

## Impact
- Affected specs: `system-task-queue`, `local-runtime`, `llm-provider-management`
- Affected code: `backend/internal/taskqueue/*`, `backend/internal/llm/*`, `backend/internal/workledger/*`, `frontend/src/views/*`

