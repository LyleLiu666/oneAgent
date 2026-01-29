## ADDED Requirements

### Requirement: Task attempt MUST run project scripts (best-effort) with evidence
系统必须 (MUST) 在 workspace 存在 `.oneagent/project.json` 时，在 task attempt 生命周期中 best-effort 执行项目脚本，并将输出作为 evidence 纳入产物。

支持的脚本字段（均为可选）：
- `setup_script`：attempt 启动前执行
- `test_script`：attempt 收尾阶段执行（用于生成 test evidence）
- `cleanup_script`：attempt 结束后执行（清理临时文件等）

系统必须 (MUST) 保证脚本执行遵循当前 attempt 的 tool permissions policy（不得绕过策略直接执行）。

当 project config 声明 `copy_files` 时，系统必须 (MUST) 在执行 `setup_script` 之前 best-effort 处理文件复制：
- 复制源必须位于 workspace root 内（不得允许绝对路径或逃逸路径）
- 复制目标为 attempt 的执行目录（默认等于 workspace root；在 worktree/隔离执行场景下可能不同）
- 若任一条目无法复制（源不存在/无权限/目标不可写），系统必须 (MUST) 让 attempt 失败并返回可操作原因（避免后续隐性失败）

#### Scenario: setup_script 执行成功并留下日志
- **GIVEN** workspace 的 `.oneagent/project.json` 包含 `setup_script`
- **WHEN** 系统启动一个新的 task attempt
- **THEN** 系统执行 `setup_script`
- **AND** 将 stdout/stderr 写入 attempt artifacts（或等价可追溯路径）

#### Scenario: setup_script 失败导致 attempt 进入失败并保留证据
- **GIVEN** workspace 的 `.oneagent/project.json` 包含 `setup_script`
- **AND** `setup_script` 退出码非 0
- **WHEN** 系统启动一个新的 task attempt
- **THEN** attempt 进入失败终态（例如 `failed`）
- **AND** attempt summary/receipt 中包含可解释原因（best-effort）
- **AND** 失败时仍保存 stdout/stderr 作为证据

#### Scenario: copy_files 在 setup_script 前被复制到执行目录
- **GIVEN** workspace 的 `.oneagent/project.json` 包含 `copy_files=[".env"]`
- **AND** workspace root 中存在 `.env`
- **WHEN** 系统启动一个新的 task attempt
- **THEN** 系统在执行 `setup_script` 之前将 `.env` 复制到 attempt 执行目录（best-effort）
- **AND** 复制过程遵循 tool permissions policy（不得绕过）

#### Scenario: copy_files 源不存在导致 attempt 失败并返回可操作错误
- **GIVEN** workspace 的 `.oneagent/project.json` 包含 `copy_files=[".env"]`
- **AND** workspace root 中不存在 `.env`
- **WHEN** 系统启动一个新的 task attempt
- **THEN** attempt 进入失败终态（例如 `failed`）
- **AND** summary/receipt 中包含“copy_files 缺失”的可操作原因（指出缺失文件路径）

#### Scenario: test_script 执行并留下测试证据
- **GIVEN** workspace 的 `.oneagent/project.json` 包含 `test_script`
- **WHEN** 一个 task attempt 的主流程执行完成并进入收尾阶段
- **THEN** 系统执行 `test_script`（best-effort）
- **AND** 将 stdout/stderr 写入 attempt artifacts（或等价可追溯路径）
- **AND** 系统在可得时将测试报告引用写入 receipt（例如 `test_report_path`）

#### Scenario: test_script 失败使 attempt 判定为 failed 并可 resume
- **GIVEN** workspace 的 `.oneagent/project.json` 包含 `test_script`
- **AND** `test_script` 退出码非 0
- **WHEN** 系统执行 attempt 的收尾阶段
- **THEN** attempt 进入失败终态（例如 `failed`）
- **AND** summary/receipt 中包含“测试失败”的可解释原因与证据引用（best-effort）
- **AND** 用户仍可通过 resume 创建新 attempt 继续（不应阻断恢复路径）

#### Scenario: cleanup_script 在 attempt 终态后执行且不改变终态
- **GIVEN** workspace 的 `.oneagent/project.json` 包含 `cleanup_script`
- **WHEN** attempt 已进入终态（succeeded/failed/canceled/...）
- **THEN** 系统执行 `cleanup_script`（best-effort）
- **AND** 将 stdout/stderr 写入 attempt artifacts（或等价可追溯路径）
- **AND** `cleanup_script` 的失败不得 (MUST NOT) 覆盖 attempt 的终态（但必须记录原因）
