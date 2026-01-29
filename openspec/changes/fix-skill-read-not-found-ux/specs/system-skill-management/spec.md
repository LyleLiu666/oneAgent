## ADDED Requirements

### Requirement: The system MUST inject a "skills help" TurnContext when user asks about available skills
当用户显式询问“有哪些/可用的 skills/技能”时，系统必须 (MUST) 在 TurnContext（volatile）注入一个“技能帮助”块，用于：
- 引导用户在 UI 中查看 skills 列表（例如 `/governance/skills`）
- 引导用户在 CLI 中检查可用性（例如 `oneagent skills status`）
- （可选）展示 Top-N（N≤10）技能名称摘要，避免模型/用户靠猜

该注入不得 (MUST NOT) 回写稳定 system prompt（避免破坏 KV cache）。

#### Scenario: User asks about available skills triggers help block
- **GIVEN** 用户输入包含“有哪些技能/skills 列表/可用 skills”等显式询问
- **WHEN** 系统生成本轮 TurnContext（volatile）注入消息
- **THEN** TurnContext 包含“技能帮助”块与下一步指引（UI + CLI）
- **AND** （可选）包含 Top-N（N≤10）技能摘要

## MODIFIED Requirements

### Requirement: Skill read tool (`skill.read`) MUST provide actionable not-found errors
当 agent 调用 `skill.read` 且请求的 skill 不存在时，工具必须 (MUST) 返回**可行动**的错误信息，而不是仅返回“not found”。该错误信息应包含（best-effort）：
- 规范化后的 skill 标识（normalized id）
- 相似候选 suggestions（≤5）
- 下一步指引（例如打开技能治理页面查看列表，或运行 `oneagent skills status` 检查可用性）

#### Scenario: skill.read not-found includes normalized id, suggestions, and next steps
- **GIVEN** skill `translator` 不存在
- **WHEN** agent 调用 `skill.read`（name 或 skill_id 为 `translator`）
- **THEN** 工具返回的错误信息包含 normalized id
- **AND** 错误信息包含 ≤5 个相似候选（best-effort）
- **AND** 错误信息包含可执行的 next steps（best-effort）
