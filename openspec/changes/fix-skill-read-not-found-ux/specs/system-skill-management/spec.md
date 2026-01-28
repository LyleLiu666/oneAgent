## ADDED Requirements

### Requirement: `skill.read` not-found MUST be actionable
系统必须 (MUST) 在 `skill.read` 无法解析指定技能时，提供可行动的错误信息，帮助用户/agent 发现正确的技能标识并继续任务，而不是只能得到“not found”。

该信息至少应包含：
- 规范化后的 skill_id（或说明如何规范化）
- 相似候选 suggestions（最多 5 个；若无候选也必须明确说明）
- next steps：如何查看当前环境的可用 skills（例如技能治理页面或 `oneagent skills status`）

#### Scenario: 未找到技能时仍给出 next steps
- **GIVEN** 当前环境存在至少一个可用 skill
- **WHEN** agent 调用 `skill.read`，输入的 name/skill_id 在技能目录中不存在
- **THEN** 返回的 tool output/错误信息包含“未找到 skill”的说明
- **AND** 包含 next steps（例如 `oneagent skills status` 或打开技能治理页面）

#### Scenario: 拼写错误时返回相似候选
- **GIVEN** 可用技能列表包含 `create-skill`
- **WHEN** agent 调用 `skill.read`，输入 `create-skll`
- **THEN** 返回的 tool output/错误信息包含 suggestions
- **AND** suggestions 中包含 `create-skill`

### Requirement: Skills help TurnContext MUST exist on explicit user request
系统必须 (MUST) 在用户显式询问“有哪些 skills/skill 可用”或等价语义时，在 TurnContext（volatile）注入一个“技能帮助”块，指引用户如何查看技能列表与可用性；并且不得回写稳定 system prompt。

该帮助块至少应包含：
- 技能治理页面入口（例如 `/governance/skills`）或等价可视化入口
- `oneagent skills status`（或等价命令）用于查看 skills 可用性与缺失依赖

#### Scenario: 用户询问 skills 列表时注入技能帮助
- **WHEN** 用户在对话中询问“你有 skill 可以使用吗/有哪些 skills 可用”
- **THEN** TurnContext（volatile）中包含“技能帮助”块
- **AND** 该块包含如何查看 skills 列表/可用性的指引

