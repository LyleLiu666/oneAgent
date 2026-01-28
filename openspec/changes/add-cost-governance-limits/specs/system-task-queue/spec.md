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

