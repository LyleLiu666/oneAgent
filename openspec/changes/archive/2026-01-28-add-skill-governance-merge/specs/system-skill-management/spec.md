## ADDED Requirements

### Requirement: Skill governance MUST support pinning a canonical skill
系统必须 (MUST) 支持将某个 skill candidate “pin”为 oneAgent personal canonical，以便在同名冲突时稳定选择该版本参与 discovery/recall。

#### Scenario: Pin shadows lower-precedence candidates
- **GIVEN** skill `foo` 在 `~/.claude/skills/foo` 存在一个版本
- **WHEN** 用户对该 candidate 执行 pin
- **THEN** 系统在 `ONEAGENT_HOME/.oneagent/skills/foo/SKILL.md` 生成 personal canonical
- **AND** 后续 `skill.Discover` 中 `foo` 的最终生效版本为 personal canonical

#### Scenario: Pin is idempotent for same source
- **GIVEN** 某 candidate 已被 pin 为 personal canonical
- **WHEN** 用户再次对同一个 candidate 执行 pin
- **THEN** 系统不会生成重复 skill
- **AND** 返回的 canonical_path 保持稳定（指向同一位置）

### Requirement: Skill governance MUST support archiving shadowed personal duplicates
系统必须 (MUST) 提供“archive shadowed personal duplicates”的治理动作，用于将同名的旧 personal skill 版本归档，以降低干扰与误召回。

#### Scenario: Archive affects only personal scope
- **GIVEN** skill `foo` 在多个来源存在 duplicates（包含 personal 与非 personal）
- **WHEN** 用户执行 archive shadowed
- **THEN** 系统只归档 personal 范围内被 shadow 的版本
- **AND** 不会尝试修改不可写来源（例如 `~/.claude`）
