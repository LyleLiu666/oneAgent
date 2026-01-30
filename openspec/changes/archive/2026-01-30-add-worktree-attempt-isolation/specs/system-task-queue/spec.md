## ADDED Requirements

### Requirement: Attempt execution root MUST be isolated when worktree mode is enabled
系统必须 (MUST) 在 worktree 模式启用时，为每个 attempt 创建并使用一个隔离的 git worktree：
- worktree 必须 (MUST) 基于 attempt 启动时的 base commit/ref 创建（记录 base SHA/branch 作为证据）
- worktree 路径必须 (MUST) 被记录到 attempt artifacts（例如 `worktree_root`）以便 UI 打开与审计
- 系统必须 (MUST) 确保 attempt 的文件写入只发生在该 worktree root 内（仍遵循 tool permissions）

#### Scenario: 创建 worktree 并记录 base SHA 与 worktree_root
- **GIVEN** workspace 是 git repo 且启用 worktree mode
- **WHEN** 系统启动一个新的 attempt
- **THEN** 系统创建一个新的 git worktree（best-effort）
- **AND** attempt artifacts 包含 `worktree_root`
- **AND** attempt artifacts 包含 `base_commit_sha`（或等价字段）

### Requirement: Worktree lifecycle MUST be managed with evidence
系统必须 (MUST) 管理 worktree 生命周期，避免“孤儿 worktree”堆积并保证可回溯：
- 系统必须 (MUST) 在 attempt 终态后按策略清理 worktree（默认清理；可配置保留用于调试）
- 当清理失败时，系统必须 (MUST) 记录失败原因并提供可操作提示（例如提示手工清理命令）
- 系统应该 (SHOULD) 提供一个 best-effort 的 orphan worktree cleanup 机制（例如启动时扫描并清理过期 worktrees）

#### Scenario: attempt 完成后按策略清理 worktree
- **GIVEN** attempt 在 worktree mode 下运行并进入终态
- **WHEN** 系统执行 attempt 收尾流程
- **THEN** 系统按策略清理或保留该 worktree
- **AND** receipt/trace 记录该决策与结果（best-effort）

