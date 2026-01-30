## ADDED Requirements

### Requirement: The system MUST record skill usage signals (best-effort)
系统必须 (MUST) 记录技能使用信号（best-effort），用于后续 staleness 判断与治理：
- `last_used_at`（最后一次被实际使用/执行的时间 best-effort）
- `used_count`（累计使用次数 best-effort）

#### Scenario: Skill usage updates last_used_at
- **GIVEN** 某 skill 被一次 task/attempt 实际使用（best-effort）
- **WHEN** 该次 attempt 进入终态并写入证据链（best-effort）
- **THEN** 该 skill 的 `last_used_at` 被更新（best-effort）

### Requirement: The system MUST support staleness detection and retirement actions (best-effort)
系统必须 (MUST) 支持对个人 skills 的 staleness 检测与淘汰治理（best-effort）：
- 系统可列出 stale candidates（best-effort）
- 用户可对 stale skill 执行 deprecate/archive，并保留 `reason`（best-effort）
- 被 deprecate/archive 的 skill 不得 (MUST NOT) 参与 discovery/recall（与现有 archived 语义一致）

#### Scenario: Deprecated skill is excluded from recall
- **GIVEN** 某 personal skill 被标记为 deprecated/archive（best-effort）
- **WHEN** 系统进行 skills discovery/recall
- **THEN** 该 skill 不出现在可用 skills 集合中（best-effort）

