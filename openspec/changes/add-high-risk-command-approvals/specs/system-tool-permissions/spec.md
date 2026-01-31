## ADDED Requirements

### Requirement: Command tools MUST support high-risk approval gating
系统必须 (MUST) 支持在 tool permissions policy 的 constraints 中为命令类工具（`bash`/`run_command`）配置 `approval=high_risk`，其语义为：
- 仅当本次请求的命令被判定为“高风险”时才进入审批门；
- `run_command` 的 `poll/cancel` 等非执行动作不得触发审批（best-effort）。

#### Scenario: Safe command does not require approval
- **GIVEN** policy 为 `bash` 设置 `approval=high_risk`
- **WHEN** LLM 调用 `bash` 执行 `ls`
- **THEN** 系统不应返回 `approval_required`（best-effort）

#### Scenario: High-risk command requires approval when in manual mode
- **GIVEN** policy 为 `bash` 设置 `approval=high_risk`
- **AND** 当前用户设置 `command_approval_mode=manual`
- **WHEN** LLM 调用 `bash` 执行 `rm -rf foo`
- **THEN** 系统必须阻断执行并返回 `approval_required`（best-effort）

#### Scenario: High-risk command is auto-approved when in auto mode
- **GIVEN** policy 为 `bash` 设置 `approval=high_risk`
- **AND** 当前用户设置 `command_approval_mode=auto`
- **WHEN** LLM 调用 `bash` 执行 `rm -rf foo`
- **THEN** 系统应自动批准并继续执行（best-effort）
- **AND** 审批记录必须可被审计（best-effort）

### Requirement: System MUST provide a user-visible toggle for command approval mode
系统必须 (MUST) 提供一个用户可见的设置项用于切换“高风险命令审批模式”，至少支持：
- `auto`：默认由系统/秘书自动审批（并留痕）
- `manual`：由用户手动审批（approve/deny）

#### Scenario: User switches approval mode to manual
- **GIVEN** 用户当前 `command_approval_mode=auto`
- **WHEN** 用户在 UI 中切换为 `manual`
- **THEN** 后续高风险命令调用应进入 `approval_required` 流程（best-effort）

