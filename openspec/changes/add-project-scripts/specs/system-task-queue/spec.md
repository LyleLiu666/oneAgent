## ADDED Requirements

### Requirement: Task attempt SHOULD run project scripts with evidence
系统应该 (SHOULD) 在 workspace 存在 `.oneagent/project.json` 时，在 task attempt 生命周期中 best-effort 执行项目脚本，并将输出作为 evidence 纳入产物。

支持的脚本字段（均为可选）：
- `setup_script`：attempt 启动前执行
- `test_script`：attempt 收尾阶段执行（用于生成 test evidence）
- `cleanup_script`：attempt 结束后执行（清理临时文件等）

系统必须 (MUST) 保证脚本执行遵循当前 attempt 的 tool permissions policy（不得绕过策略直接执行）。

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

