# system-skill-management Specification

## Purpose
TBD - created by archiving change enable-skills-usage. Update Purpose after archive.
## Requirements
### Requirement: 多来源技能发现 (Multi-Source Skill Discovery)
系统必须 (MUST) 从以下位置发现技能文件（`SKILL.md`），并支持以“技能目录包”的方式组织（目录内包含 `SKILL.md`，可选包含 `scripts/`、`references/` 等）。系统应对以上来源取并集后按技能 `name` 去重（同名冲突按稳定规则消解）：
1) `<workspace>/.oneagent/skills/**/SKILL.md`
2) `<workspace>/skills/**/SKILL.md`
3) `<workspace>/.claude/skills/**/SKILL.md`
4) `~/.claude/skills/**/SKILL.md`
5) `~/.codex/skills/**/SKILL.md`
6) 内置 skills（随二进制发布，path 形如 `builtin:skills/<id>/SKILL.md`）

#### Scenario: 发现 ~/.claude skills
- **GIVEN** 用户主目录存在 `~/.claude/skills/translator/SKILL.md`
- **WHEN** 系统加载技能目录
- **THEN** 技能列表中包含该技能（source=`.claude`）

#### Scenario: 发现 <workspace>/.oneagent skills
- **GIVEN** 工作区根目录存在 `<workspace>/.oneagent/skills/code-review/SKILL.md`
- **WHEN** 系统加载技能目录
- **THEN** 技能列表中包含该技能（source=`.oneagent`）

#### Scenario: 发现 <workspace>/skills
- **GIVEN** 工作区根目录存在 `<workspace>/skills/tech-spec-architect/SKILL.md`
- **WHEN** 系统加载技能目录
- **THEN** 技能列表中包含该技能（source=`.workspace`）

#### Scenario: 发现 <workspace>/.claude skills
- **GIVEN** 工作区根目录存在 `<workspace>/.claude/skills/translator/SKILL.md`
- **WHEN** 系统加载技能目录
- **THEN** 技能列表中包含该技能（source=`.claude`）

#### Scenario: 发现 ~/.codex skills
- **GIVEN** 用户主目录存在 `~/.codex/skills/git-auto-commit/SKILL.md`
- **WHEN** 系统加载技能目录
- **THEN** 技能列表中包含该技能（source=`.codex`）

#### Scenario: 冲突时以 .oneagent 覆盖 <workspace>/skills
- **GIVEN** `<workspace>/skills/translator/SKILL.md` 与 `<workspace>/.oneagent/skills/translator/SKILL.md` 同时存在
- **WHEN** 系统加载技能目录
- **THEN** 仅保留 `.oneagent` 版本作为最终生效技能（同名覆盖）

#### Scenario: 冲突时以 <workspace>/skills 覆盖 <workspace>/.claude
- **GIVEN** `<workspace>/.claude/skills/translator/SKILL.md` 与 `<workspace>/skills/translator/SKILL.md` 同时存在
- **WHEN** 系统加载技能目录
- **THEN** 仅保留 `<workspace>/skills` 版本作为最终生效技能（同名覆盖）

#### Scenario: 冲突时以 <workspace>/.claude 覆盖 ~/.claude
- **GIVEN** `~/.claude/skills/translator/SKILL.md` 与 `<workspace>/.claude/skills/translator/SKILL.md` 同时存在
- **WHEN** 系统加载技能目录
- **THEN** 仅保留 `<workspace>/.claude` 版本作为最终生效技能（同名覆盖）

#### Scenario: 冲突时以 .claude 覆盖 .codex
- **GIVEN** `~/.codex/skills/translator/SKILL.md` 与 `~/.claude/skills/translator/SKILL.md` 同时存在
- **WHEN** 系统加载技能目录
- **THEN** 仅保留 `.claude` 版本作为最终生效技能（同名覆盖）

#### Scenario: 冲突时以 workspace 覆盖内置 skills
- **GIVEN** 内置 skills 中存在 `create-skill`，且 `<workspace>/.oneagent/skills/create-skill/SKILL.md` 同时存在
- **WHEN** 系统加载技能目录
- **THEN** 仅保留 workspace 版本作为最终生效技能（同名覆盖）

#### Scenario: ~/.claude skills 为 symlink 目录
- **GIVEN** `~/.claude/skills/code-review-excellence` 是一个 symlink，指向某个真实目录且该目录内包含 `SKILL.md`
- **WHEN** 系统加载技能目录
- **THEN** 技能列表中包含该技能（不得因 symlink 而漏掉）

### Requirement: 召回式技能推荐 (Recall Recommendation)
系统必须 (MUST) 在聊天开始前使用“技能召回工具”从全量技能中召回 Top-8，并从中选择 Top-1 作为“推荐技能”（或无推荐），提供给主 agent。

#### Scenario: 海量技能时仅召回 Top-8 且不注入全量列表
- **GIVEN** 可用技能数量为 1000+
- **WHEN** 构建本轮 TurnContext（volatile）的“技能建议”注入消息
- **THEN** 技能召回阶段只处理 Top-8 候选（固定为 8）
- **THEN** TurnContext 不得出现“全量技能列表”或“把所有技能都列出”的行为
- **THEN** 稳定 system prompt 不得被回写（避免破坏 KV cache）

#### Scenario: 无推荐技能时不注入推荐块
- **GIVEN** 技能召回工具返回空结果（无候选）
- **WHEN** 构建本轮 TurnContext（volatile）的“技能建议”注入消息
- **THEN** TurnContext 不包含“## 技能建议”块（或包含但明确说明无推荐技能）
- **THEN** 稳定 system prompt 不得被回写（避免破坏 KV cache）

### Requirement: 显式指定技能名称（语义）(Explicit Skill Selection, Semantic)
系统必须 (MUST) 保留“按技能名称指定”的使用方式：当用户在自然语言中明确表达要使用某个技能（通常会提到技能名称）时，系统应优先解析并推荐该技能（可跳过召回/选择流程或作为 override）。

#### Scenario: 用户自然语言指定技能名优先生效
- **GIVEN** 用户输入包含 “请使用 code-review-excellence 这个技能”
- **WHEN** 构建本轮 TurnContext（volatile）的“技能建议”注入消息
- **THEN** 系统将 `code-review-excellence` 作为推荐技能（前提：该技能存在）
- **THEN** 稳定 system prompt 不得被回写（避免破坏 KV cache）

### Requirement: 中文上下文注入 (Chinese Context Injection)
系统必须 (MUST) 通过 TurnContext（volatile）消息使用中文提示词注入“推荐技能摘要”，以符合用户偏好，并引导 agent 在使用前先通过 `skill.read` 读取该技能的 `SKILL.md`；系统不得 (MUST NOT) 回写稳定 system prompt。

#### Scenario: 中文提示词标题 (Scenario: Chinese Prompt Header)
- **GIVEN** 系统产生一个推荐技能
- **WHEN** 生成 TurnContext（volatile）注入消息时
- **THEN** 它**必须**包含 "## 技能建议"
- **THEN** 它**必须**包含 "`skill.read`" 或等价表述（强调先通过 skill 读取工具读到 `SKILL.md` 再执行）

#### Scenario: 列表中的技能描述 (Scenario: Skill Description in List)
- **GIVEN** 系统推荐技能 "Translator"，描述为 "Expert in translation"
- **WHEN** 生成 TurnContext（volatile）注入消息时
- **THEN** 它**必须**包含 "- Translator: Expert in translation"
- **THEN** 它**必须**包含“如何读取该技能”的提示（例如包含 `skill.read("Translator")` 或等价表述）

### Requirement: Skill 读取工具（`skill.read`）
系统必须 (MUST) 提供一个 skill 读取工具（例如 `skill.read`），使 agent 可仅凭技能名称/ID 读取对应技能的 `SKILL.md` 原文；该原文必须作为工具调用输出（tool output）进入对话上下文，以便 agent 在后续思考与回复中遵循该 Skill 的指令。

该工具必须 (MUST) 按技能发现的同名覆盖规则返回“最终生效版本”（例如 `<workspace>/.oneagent` > `<workspace>/skills` > `<workspace>/.claude` > `~/.claude` > `~/.codex` > `.builtin`），且不得要求调用方提供文件路径。

#### Scenario: 仅凭技能名称读取 SKILL.md
- **GIVEN** 技能来源中存在技能 "Translator"
- **WHEN** agent 调用 `skill.read("Translator")`
- **THEN** 系统返回该技能的 `SKILL.md` 原文作为 tool output

#### Scenario: 同名冲突时返回最终生效版本
- **GIVEN** `~/.claude/skills/translator/SKILL.md` 与 `<workspace>/.oneagent/skills/translator/SKILL.md` 同时存在
- **WHEN** agent 调用 `skill.read("translator")`
- **THEN** 系统返回 `.oneagent` 版本的 `SKILL.md` 原文作为 tool output
