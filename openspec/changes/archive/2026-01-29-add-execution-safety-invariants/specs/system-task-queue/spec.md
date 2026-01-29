## ADDED Requirements

### Requirement: Mutating attempts MUST have a rollback boundary
系统必须 (MUST) 为任何可能修改用户资产的 task attempt 建立一个“可回退边界”（rollback boundary），以满足“可修改资产必须可回退”的底线。

该边界至少应覆盖 workspace 内的文件系统改动，并满足：
- attempt 开始时生成可恢复的 checkpoint（例如 worktree/base commit、git snapshot、或等价机制）
- attempt 终态后用户可触发 rollback，将 workspace 恢复到 checkpoint 状态（best-effort）
- rollback 不得删除该 attempt 的证据链（trace/findings/diff artifacts 仍需保留用于复盘）

#### Scenario: User rolls back a failed attempt and workspace is restored
- **GIVEN** 一个 attempt 产生了 workspace 文件改动并最终 `failed`
- **WHEN** 用户对该 attempt 触发 rollback
- **THEN** workspace 文件状态被恢复到该 attempt 开始前的 checkpoint（best-effort）
- **AND** 该 attempt 的 artifacts/trace 仍可被查询与打开

### Requirement: Rollback MUST be auditable and idempotent
系统必须 (MUST) 将 rollback 作为一等事件记录到审计证据链，并确保重复触发不会产生额外破坏：
- rollback 事件记录包含 `attempt_id`、checkpoint 引用、操作者（单用户可为 implicit principal）、时间、以及结果（success/fail + reason）
- 对同一 attempt 重复触发 rollback 时，系统应返回“已回退”或等价的幂等结果（best-effort）

#### Scenario: Repeated rollback is idempotent
- **GIVEN** 用户已成功对某 attempt 执行过一次 rollback
- **WHEN** 用户再次对同一 attempt 触发 rollback
- **THEN** 系统返回幂等结果（不再次破坏 workspace）
- **AND** 事件日志包含一次可解释的重复触发记录（best-effort）

