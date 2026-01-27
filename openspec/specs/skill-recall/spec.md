# skill-recall Specification

## Purpose
TBD - created by archiving change enable-skills-usage. Update Purpose after archive.
## Requirements
### Requirement: 基础召回独立于主 agent 且无需外部网络
技能召回工具必须 (MUST) 在本地运行，独立于主 agent（主对话 LLM 推理），并且不得依赖外部网络服务来完成“基础召回”（Top-8 候选生成）。

#### Scenario: 离线可用
- **GIVEN** 运行环境不可访问互联网
- **WHEN** 执行技能召回（基于本地文件）
- **THEN** 工具仍可返回技能候选列表（不因网络不可用而失败）

### Requirement: 支持多来源发现并基于本地文件召回（grep/ripgrep）
技能召回工具必须 (MUST) 能从以下来源发现技能并进行召回（对以上来源取并集后按 `name` 去重）：
1) `<workspace>/.oneagent/skills`
2) `<workspace>/skills`
3) `<workspace>/.claude/skills`
4) `ONEAGENT_HOME/.oneagent/skills`（oneAgent-managed personal skills，active）
5) `~/.claude/skills`
6) `~/.codex/skills`
7) 内置 skills（随二进制发布，path 形如 `builtin:skills/<id>/SKILL.md`）

#### Scenario: 个人 skills 可被召回
- **GIVEN** `ONEAGENT_HOME/.oneagent/skills/my-sop/SKILL.md` 存在且内容匹配 query
- **WHEN** 执行技能召回（基于本地文件）
- **THEN** 工具返回结果包含该技能

#### Scenario: 归档的个人 skills 不参与召回
- **GIVEN** `ONEAGENT_HOME/.oneagent/skills-archived/old-sop/SKILL.md` 存在且内容匹配 query
- **WHEN** 执行技能召回（基于本地文件）
- **THEN** 工具返回结果不包含该技能（archived skills 不得参与 recall）

### Requirement: 召回接口固定返回 Top-8 并返回可用字段
技能召回工具必须 (MUST) 提供一个查询接口（CLI 或库 API），输入 query，输出 Top-8 技能候选列表，并包含至少以下字段：
`skill_id`, `name`, `description`, `source`, `path`, `score`

#### Scenario: 查询返回 Top-8 并包含必要字段
- **GIVEN** 技能来源中存在技能 "Translator"（description 包含 "translation"）
- **WHEN** 以 query="translation" 执行召回
- **THEN** 返回结果数量不超过 8
- **THEN** 结果中包含 "Translator"
- **THEN** 每条结果包含 `skill_id/name/description/source/path/score`

### Requirement: 召回结果顺序稳定
在技能来源内容不变的前提下，技能召回工具必须 (MUST) 对同一 query 返回稳定的排序结果（避免同分随机顺序导致提示词抖动）。

#### Scenario: 相同输入得到相同输出顺序
- **GIVEN** 技能来源内容未变化
- **WHEN** 连续两次以相同 query 执行召回
- **THEN** 返回结果的排序顺序一致

### Requirement: 支持语义显式指定技能名的解析
技能召回工具必须 (MUST) 支持通过“技能名称”直接解析技能（无需依赖特殊语法），用于处理用户自然语言中明确表达“要使用某技能”的情况。

#### Scenario: 自然语言指定技能名可解析
- **GIVEN** 技能来源中存在技能 `code-review-excellence`
- **WHEN** 输入 query="请使用 code-review-excellence 这个技能，帮我 review 这个 PR"
- **THEN** 工具可以将 `code-review-excellence` 解析为候选/推荐技能（可作为 override）

