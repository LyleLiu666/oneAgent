## MODIFIED Requirements
### Requirement: 多来源技能发现 (Multi-Source Skill Discovery)
系统必须 (MUST) 从以下位置发现技能文件（`SKILL.md`），并支持以“技能目录包”的方式组织（目录内包含 `SKILL.md`，可选包含 `scripts/`、`references/` 等）。系统应对以上来源取并集后按技能 `name` 去重（同名冲突按稳定规则消解）：
1) `<workspace>/.oneagent/skills/**/SKILL.md`
2) `<workspace>/skills/**/SKILL.md`
3) `<workspace>/.claude/skills/**/SKILL.md`
4) `ONEAGENT_HOME/.oneagent/skills/**/SKILL.md`（oneAgent-managed personal skills）
5) `~/.claude/skills/**/SKILL.md`
6) `~/.codex/skills/**/SKILL.md`
7) 内置 skills（随二进制发布，path 形如 `builtin:skills/<id>/SKILL.md`）

#### Scenario: 发现 ONEAGENT_HOME 下的个人 skills
- **GIVEN** `ONEAGENT_HOME/.oneagent/skills/my-sop/SKILL.md` 存在
- **WHEN** 系统加载技能目录
- **THEN** 技能列表中包含该技能（source=`.oneagent_home` 或等价 source）

#### Scenario: workspace skills 覆盖个人 skills
- **GIVEN** `ONEAGENT_HOME/.oneagent/skills/translator/SKILL.md` 与 `<workspace>/.oneagent/skills/translator/SKILL.md` 同时存在
- **WHEN** 系统加载技能目录
- **THEN** 仅保留 `<workspace>/.oneagent` 版本作为最终生效技能（同名覆盖）

