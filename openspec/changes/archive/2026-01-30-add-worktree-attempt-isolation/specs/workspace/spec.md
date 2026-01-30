## ADDED Requirements

### Requirement: Workspace MUST support optional worktree execution mode for attempts
系统必须 (MUST) 支持为 workspace 配置一种可选的 task attempt 执行模式 `worktree`（仅在 workspace 是 git repo 时生效）。

当 `worktree` 模式启用时，系统必须 (MUST) 为每个 attempt 选择一个独立的“执行根目录”（worktree root），并把它作为文件/搜索/命令工具的默认作用域与写入边界。

当 workspace 不是 git repo 时，系统必须 (MUST) 明确返回可操作错误或按配置退化到 `workspace` 模式（不得 silent fallback）。

#### Scenario: git workspace 启用 worktree mode 后 attempt 使用独立执行根目录
- **GIVEN** workspace 是 git repo
- **AND** workspace 启用了 attempt 执行模式 `worktree`
- **WHEN** 系统创建一个新的 task attempt
- **THEN** 该 attempt 的执行根目录为一个独立 worktree 路径（不等于 workspace root）
- **AND** 文件/命令工具默认在该 worktree 路径下执行

#### Scenario: non-git workspace 启用 worktree mode 返回可操作错误
- **GIVEN** workspace 不是 git repo
- **AND** workspace 启用了 attempt 执行模式 `worktree`
- **WHEN** 系统尝试创建一个新的 task attempt
- **THEN** 系统返回清晰错误（指出“workspace 非 git repo，无法创建 worktree”）

