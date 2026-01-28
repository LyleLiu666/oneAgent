## ADDED Requirements

### Requirement: Task limits MUST support cost/token budgets
系统必须 (MUST) 支持为 task 的 `limits` 增加可选的预算字段，用于治理长任务的成本：
- `max_total_tokens`（或等价字段）
- `max_cost_usd`（或等价字段）

#### Scenario: Create task with token budget
- **WHEN** 用户创建 task 并提供 `limits.max_total_tokens`
- **THEN** 系统持久化该预算并在 task 查询中返回

#### Scenario: Budget exceeded stops the attempt
- **GIVEN** 某 attempt 的累计 tokens/cost 即将超过预算
- **WHEN** 系统继续执行下一次 LLM 调用
- **THEN** 系统停止该 attempt 并进入终态
- **AND** summary/trace 中包含“预算耗尽”的可解释原因

### Requirement: Task attempt MUST expose aggregated usage
系统必须 (MUST) 在 task attempt 上暴露累计 usage，以便用户理解“跑了多少、花了多少”（best-effort）：
- `attempt.usage.calls`
- `attempt.usage.prompt_tokens`
- `attempt.usage.completion_tokens`
- `attempt.usage.total_tokens`
- `attempt.usage.cost_usd`（如果可得）

#### Scenario: Get task returns attempt usage
- **GIVEN** 某 attempt 已产生至少一次 LLM 调用
- **WHEN** 用户查询 task
- **THEN** 返回的 attempt 包含 `usage.total_tokens > 0`

#### Scenario: Budget exceeded attempt is resumable
- **GIVEN** 某 attempt 因预算耗尽而进入终态（例如 `status=limit_exceeded`）
- **WHEN** 用户对该 task 执行 resume
- **THEN** 系统创建新的 attempt 并允许继续执行
