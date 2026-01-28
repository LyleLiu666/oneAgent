## ADDED Requirements

### Requirement: Skill governance workbench MUST support listing and archiving
系统必须 (MUST) 提供一个治理入口，用于列出当前可用技能，并支持对个人技能执行归档操作。

#### Scenario: 列出 skills
- **WHEN** 用户打开 skill 治理页面
- **THEN** 系统返回可发现的 skills 列表（至少包含 id/name/description/source/path）

#### Scenario: 归档 oneAgent personal skill
- **GIVEN** 某 skill 来源为 oneAgent personal skills
- **WHEN** 用户执行 archive
- **THEN** skill 被移动到 archived 位置
- **AND** 后续列出 skills 时不再包含该 skill

