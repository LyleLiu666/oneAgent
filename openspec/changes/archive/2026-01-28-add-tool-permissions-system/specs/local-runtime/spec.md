## ADDED Requirements

### Requirement: Global tool disable switch MUST be respected
系统必须 (MUST) 保留 `ONEAGENT_DISABLE_TOOL_*` 作为全局 kill-switch，并在所有权限策略之前生效。

#### Scenario: Global disable overrides policy
- **GIVEN** `ONEAGENT_DISABLE_TOOL_WRITE_FILE=1`
- **WHEN** 任何 principal 请求使用 `write_file`
- **THEN** 系统拒绝该工具调用并返回“全局禁用”原因

