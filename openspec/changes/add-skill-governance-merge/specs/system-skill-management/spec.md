## ADDED Requirements

### Requirement: Skill governance MUST support pinning a canonical skill
系统必须 (MUST) 支持将某个 skill candidate “pin”为 oneAgent personal canonical，以便在同名冲突时稳定选择该版本参与 discovery/recall。

#### Scenario: Pin shadows lower-precedence candidates
- **GIVEN** skill `foo` 在 `~/.claude/skills/foo` 存在一个版本
- **WHEN** 用户对该 candidate 执行 pin
- **THEN** 系统在 `ONEAGENT_HOME/.oneagent/skills/foo/SKILL.md` 生成 personal canonical
- **AND** 后续 `skill.Discover` 中 `foo` 的最终生效版本为 personal canonical

