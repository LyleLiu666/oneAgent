## ADDED Requirements

### Requirement: Subagent MUST inherit and not exceed parent permissions
系统必须 (MUST) 确保子 Agent 的工具权限不超过父上下文；子 Agent 仅可使用父级允许的工具集合与约束。

#### Scenario: Subagent cannot escalate permissions
- **GIVEN** 父上下文不允许 `bash`
- **WHEN** 子 Agent 尝试调用 `bash`
- **THEN** 系统拒绝并返回权限错误

