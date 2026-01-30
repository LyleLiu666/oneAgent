## ADDED Requirements

### Requirement: Receipt MUST reference worktree evidence when used
系统必须 (MUST) 在 attempt 使用 worktree mode 时，将 worktree 相关证据写入 Receipt，以便复盘与恢复：
- `worktree_root`（路径）
- `base_commit_sha`（或等价字段）

#### Scenario: Receipt includes worktree_root and base_commit_sha
- **GIVEN** 某次 attempt 在 worktree mode 下运行且 artifacts 中包含 `worktree_root/base_commit_sha`
- **WHEN** 系统持久化该次交付的 receipt
- **THEN** receipt artifacts 包含 `worktree_root`
- **AND** receipt artifacts 包含 `base_commit_sha`

