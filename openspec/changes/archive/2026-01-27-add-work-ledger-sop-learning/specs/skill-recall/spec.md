## MODIFIED Requirements
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
