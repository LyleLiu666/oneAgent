## MODIFIED Requirements

### Requirement: 中文上下文注入 (Chinese Context Injection)
系统必须 (MUST) 通过 TurnContext（volatile）消息使用中文提示词注入“推荐技能摘要”，以符合用户偏好，并引导 agent 在使用前先通过 `skill_read` 读取该技能的 `SKILL.md`；系统不得 (MUST NOT) 回写稳定 system prompt。

系统必须 (MUST) 在过渡期内兼容历史名称 `skill.read`（alias，best-effort）。

#### Scenario: 中文提示词标题 (Scenario: Chinese Prompt Header)
- **GIVEN** 系统产生一个推荐技能
- **WHEN** 生成 TurnContext（volatile）注入消息时
- **THEN** 它**必须**包含 "## 技能建议"
- **THEN** 它**必须**包含 "`skill_read`" 或等价表述（强调先通过 skill 读取工具读到 `SKILL.md` 再执行）

#### Scenario: 列表中的技能描述 (Scenario: Skill Description in List)
- **GIVEN** 系统推荐技能 "Translator"，描述为 "Expert in translation"
- **WHEN** 生成 TurnContext（volatile）注入消息时
- **THEN** 它**必须**包含 "- Translator: Expert in translation"
- **THEN** 它**必须**包含“如何读取该技能”的提示（例如包含 `skill_read(name="Translator")` 或等价表述）

### Requirement: Skill 读取工具（`skill.read`）
系统必须 (MUST) 提供一个 skill 读取工具（canonical name 为 `skill_read`；并兼容 `skill.read` alias），使 agent 可仅凭技能名称/ID 读取对应技能的 `SKILL.md` 原文；该原文必须作为工具调用输出（tool output）进入对话上下文，以便 agent 在后续思考与回复中遵循该 Skill 的指令。

该工具必须 (MUST) 按技能发现的同名覆盖规则返回“最终生效版本”（例如 `<workspace>/.oneagent` > `<workspace>/skills` > `<workspace>/.claude` > `~/.claude` > `~/.codex` > `.builtin`），且不得要求调用方提供文件路径。

#### Scenario: 仅凭技能名称读取 SKILL.md
- **GIVEN** 技能来源中存在技能 "Translator"
- **WHEN** agent 调用 `skill_read(name="Translator")`
- **THEN** 系统返回该技能的 `SKILL.md` 原文作为 tool output

#### Scenario: 同名冲突时返回最终生效版本
- **GIVEN** `~/.claude/skills/translator/SKILL.md` 与 `<workspace>/.oneagent/skills/translator/SKILL.md` 同时存在
- **WHEN** agent 调用 `skill_read(name="translator")`
- **THEN** 系统返回 `.oneagent` 版本的 `SKILL.md` 原文作为 tool output

