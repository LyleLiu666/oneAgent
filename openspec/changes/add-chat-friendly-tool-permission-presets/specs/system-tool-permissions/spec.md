## ADDED Requirements
### Requirement: Tool permissions MUST provide user-facing simple execution modes
系统必须 (MUST) 在保留完整 `ToolPolicy` 能力的前提下，提供一层面向普通用户的“简单执行模式”（simple modes），至少包括：
- `readonly`：只读查看
- `sandbox_coding`：沙箱开发
- `host_full`：本机执行
- `custom`：无法安全识别为上述预设的自定义策略

系统必须 (MUST) 提供可读取当前 simple mode 的接口，并将当前真实 policy 映射到上述模式之一（best-effort）。

#### Scenario: User can read the current simple execution mode
- **GIVEN** 当前 principal 已存在一份工具权限策略
- **WHEN** 客户端请求 simple mode 状态
- **THEN** 系统返回当前模式（`readonly|sandbox_coding|host_full|custom`）
- **AND** 同时返回当前 policy snapshot（best-effort）

#### Scenario: Custom policy is surfaced as custom instead of a wrong preset
- **GIVEN** 当前 principal 的 policy 不是系统定义的简单预设之一
- **WHEN** 客户端请求 simple mode 状态
- **THEN** 系统返回 `custom`
- **AND** 不得把该 policy 错误显示成更高或更低权限的预设

#### Scenario: Built-in default policy is classified by effective behavior instead of raw JSON equality
- **GIVEN** 当前 principal 使用系统默认 policy（best-effort）
- **WHEN** 客户端请求 simple mode 状态
- **THEN** 系统按当前平台上的有效执行语义返回合理的 simple mode（best-effort）
- **AND** 不要求 policy JSON 必须与某个模板逐字完全一致

### Requirement: Simple execution mode APIs MUST be scoped to the current authenticated principal
系统必须 (MUST) 将 simple execution mode 视为“当前用户的自助权限入口”，而不是管理员跨 principal 治理接口：
- simple API 只能读取和修改当前认证 principal 的权限状态
- 不得通过 simple API 修改其它 principal 的 policy
- 管理员跨 principal 的治理继续走现有 admin policy 接口（best-effort）

#### Scenario: User cannot change another principal via simple mode API
- **GIVEN** 当前请求的认证 principal 为 `alice`
- **WHEN** 客户端尝试通过 simple mode API 为 `bob` 切换执行模式（best-effort）
- **THEN** 系统拒绝该请求或忽略外部 `principal_id`
- **AND** 最终只能作用于 `alice`

### Requirement: Simple execution mode changes MUST be explicit, safe, and auditable
系统必须 (MUST) 允许用户通过 simple mode 直接切换执行权限，而无需编辑 JSON；但系统不得 (MUST NOT) 静默提权：
- 所有 simple mode 切换都必须由明确用户操作触发
- 所有切换都必须写入审计证据（source / principal / old policy / new policy / changed_at）
- `host_full` 必须有更高风险提示（best-effort）

#### Scenario: User applies a simple execution mode without editing JSON
- **GIVEN** 用户希望把当前执行权限切到 `sandbox_coding`
- **WHEN** 客户端调用 simple mode 应用接口
- **THEN** 系统将该模式映射成真实 `ToolPolicy` 并持久化
- **AND** 返回新的 simple mode 状态与 policy snapshot（best-effort）

#### Scenario: Applying a simple mode produces an audit record
- **GIVEN** 用户通过简单权限入口切换执行模式
- **WHEN** 系统完成保存
- **THEN** 系统记录本次切换的审计证据（source / principal / old policy hash / new policy hash / selected mode）

#### Scenario: Audit record is durable
- **GIVEN** 用户通过简单权限入口切换执行模式
- **WHEN** 服务端写入审计信息
- **THEN** 该审计记录会持久化保存（best-effort）
- **AND** 不得只依赖临时控制台日志

### Requirement: Sandbox coding mode MUST resolve to a hard-boundary runtime or fail actionably
系统必须 (MUST) 将 `sandbox_coding` 解释为“允许 coding profile，但必须运行在硬边界隔离环境中”。

系统可以 (MAY) 由后端按环境选择 `native` 或 `docker`，但若当前环境不存在可用的硬边界执行后端，系统必须 (MUST) 返回可操作错误，而不是静默降级到 host 或只读。

#### Scenario: Sandbox coding resolves to an available hard-boundary mode
- **GIVEN** 当前环境支持 `native` 或 `docker`
- **WHEN** 用户应用 `sandbox_coding`
- **THEN** 系统保存的真实 policy 对命令类工具使用 `command_profile=coding`
- **AND** 同时要求 `sandbox_mode` 为可用的硬边界模式（best-effort）

#### Scenario: Sandbox coding fails with actionable guidance when no hard boundary is available
- **GIVEN** 当前环境不支持 `native` 且也不可用 `docker`
- **WHEN** 用户应用 `sandbox_coding`
- **THEN** 系统拒绝保存该 simple mode
- **AND** 返回可操作错误，说明如何启用隔离环境或改用其它模式

### Requirement: Simple execution mode changes MUST NOT relax secretary read-only guarantees
系统必须 (MUST) 保持秘书 agent 的默认只读边界不变；simple execution mode 切换只影响 worker / 普通聊天 / 后续 task attempt，不得把秘书自身提权为可写。

#### Scenario: Secretary remains read-only after switching to a stronger simple mode
- **GIVEN** 用户将当前 simple mode 切换为 `sandbox_coding` 或 `host_full`
- **WHEN** 秘书后续继续执行 triage / 汇报（best-effort）
- **THEN** 秘书仍使用只读策略（best-effort）
- **AND** 不得因此获得写文件或命令写能力

### Requirement: Simple execution mode changes MUST only affect future executions
系统必须 (MUST) 明确 simple execution mode 的生效边界：
- 新的 worker chat 回合使用新策略（best-effort）
- 新建 task attempt 或 resume 后产生的新 attempt 使用新策略（best-effort）
- 已经在运行中的 attempt 保持其既有 `policy_snapshot` 不变

#### Scenario: Running attempt keeps its original policy snapshot
- **GIVEN** 某 task attempt 已开始运行并已解析出 `policy_snapshot`
- **WHEN** 用户切换 simple execution mode
- **THEN** 当前 attempt 继续使用原有 `policy_snapshot`
- **AND** 新策略仅影响之后的新执行（best-effort）

### Requirement: Tool permissions UX MUST use progressive disclosure
系统必须 (MUST) 将 simple modes 作为默认权限入口，而将 JSON policy editor 保留为高级兜底能力（best-effort）。

#### Scenario: User can change common permission modes without opening the JSON editor
- **GIVEN** 用户只想切换到常见执行模式（例如只读或本机执行）
- **WHEN** 用户进入工具权限 UI（或等价入口）
- **THEN** 用户可以直接通过 simple mode 完成切换
- **AND** 不需要先编辑 JSON

#### Scenario: Advanced JSON remains available for custom policies
- **GIVEN** 用户需要表达系统预设之外的自定义策略
- **WHEN** 用户进入高级设置
- **THEN** 系统仍提供 JSON policy editor（best-effort）
- **AND** simple mode 状态显示为 `custom`（best-effort）
