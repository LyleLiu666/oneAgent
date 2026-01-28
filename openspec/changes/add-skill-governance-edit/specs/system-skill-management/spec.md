## ADDED Requirements

### Requirement: Personal skills MUST be viewable and editable via API
系统必须 (MUST) 为 oneAgent personal skills 提供“查看与编辑 SKILL.md”的能力，作为治理工作台的基础。

#### Scenario: 获取 skill 详情包含 SKILL.md 与 sha
- **GIVEN** 某 skill 为 oneAgent personal skills
- **WHEN** 客户端请求 `GET /api/skills/:id`
- **THEN** 响应包含 `skill_md`（完整 SKILL.md 原文）与 `sha256`（用于 OCC）

#### Scenario: 使用 OCC 更新 SKILL.md
- **GIVEN** 客户端已读取到 `sha256`
- **WHEN** 客户端请求 `PUT /api/skills/:id` 并携带 `expected_sha256`
- **THEN** 当文件未变化时更新成功
- **AND** 当文件已变化时必须拒绝并返回明确错误

