## ADDED Requirements

### Requirement: Skill governance MUST expose duplicates candidates
系统必须 (MUST) 提供一个治理 API，用于列出“同名冲突”的 skill candidates，并标记每组中最终生效版本。

#### Scenario: 列出 duplicates
- **GIVEN** 同一 `skill_id` 在不同来源存在多个版本
- **WHEN** 客户端请求 `GET /api/skills/duplicates`
- **THEN** 响应按 `skill_id` 分组返回 candidates
- **AND** 每个 candidate 至少包含 `source/path/archivable/effective`

#### Scenario: effective 选择与 Discover 一致
- **GIVEN** `<workspace>/.oneagent/skills/foo` 与 `ONEAGENT_HOME/.oneagent/skills/foo` 同时存在
- **WHEN** 客户端请求 `GET /api/skills/duplicates`
- **THEN** `<workspace>/.oneagent` 版本标记为 `effective=true`
- **AND** 其他版本标记为 `effective=false`

