## ADDED Requirements

### Requirement: Default cost/token limits MUST be configurable
系统必须 (MUST) 允许通过环境变量为 task queue 配置默认的 cost/token 预算与 cap，以便团队治理（例如在部门内控制预算）。

#### Scenario: Default budgets applied when omitted
- **GIVEN** 系统配置了默认 `max_total_tokens` 与 `max_cost_usd`
- **WHEN** 用户创建 task 且未提供这些字段
- **THEN** 系统自动填充默认预算
