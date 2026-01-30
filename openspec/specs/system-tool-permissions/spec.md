# system-tool-permissions Specification

## Purpose
TBD - created by archiving change add-tool-permissions-system. Update Purpose after archive.
## Requirements
### Requirement: System MUST define a per-principal tool permissions policy
系统必须 (MUST) 支持按 `principal_id` 定义工具权限策略，策略由多条规则组成，并支持 `allow/deny` 效果与约束（constraints）。

#### Scenario: Principal policy is resolved
- **GIVEN** `principal_id=alice` 绑定了自定义策略
- **WHEN** 系统为该 principal 生成可用工具列表
- **THEN** 仅返回策略允许的工具
- **AND** 被拒绝的工具不可被执行

### Requirement: Tool permissions MUST be enforced at mount and execution
系统必须 (MUST) 在工具挂载（LLM tool list）与工具执行两个层面执行权限校验；即便 LLM 通过缓存/猜测触发工具调用，也必须被拒绝。

#### Scenario: Execution denied even if tool was cached
- **GIVEN** 某 tool 已被 LLM 缓存或猜到名称
- **WHEN** 该 tool 在执行时被策略拒绝
- **THEN** 系统返回明确的权限错误（包含拒绝原因）

### Requirement: Permissions MUST be explainable and auditable
系统必须 (MUST) 在 allow/deny 决策中记录“匹配规则/来源/原因”，并可用于审计（日志/trace）。

#### Scenario: Deny response includes reason
- **GIVEN** 某 tool 被策略拒绝
- **WHEN** 系统返回错误
- **THEN** 错误中包含“由哪条规则拒绝”的解释信息（rule id 或等价）

### Requirement: Command tools MUST support profiles (readonly/dev/full)
系统必须 (MUST) 为 `bash`/`run_command` 提供可配置的命令 profile（`readonly/dev/coding/full`），默认使用最小权限集合，并优先采用 allowlist。

其中 `coding` profile 仅用于 coding agent 场景，必须 (MUST) 结合强隔离 sandbox（例如 `sandbox_mode=docker`）使用；当环境不满足时必须 fail-closed 并返回可操作错误（best-effort）。

#### Scenario: readonly profile blocks interpreters
- **GIVEN** 当前 profile 为 `readonly`
- **WHEN** 调用 `bash` 执行 `python`/`node` 等解释器
- **THEN** 系统拒绝该命令并返回可解释错误

#### Scenario: coding profile requires docker sandbox
- **GIVEN** 当前 profile 为 `coding` 且 policy 要求 `sandbox_mode=docker`
- **WHEN** 调用 `run_command` 执行 `git status`（best-effort）
- **THEN** 系统在 docker sandbox 中执行或返回可操作的环境缺失错误（best-effort）

### Requirement: Command tools MUST support sandbox_mode constraints
系统必须 (MUST) 在 tool permissions policy 的 constraints 中支持对命令类工具（`bash`/`run_command`）指定 `sandbox_mode`（例如 `none`/`docker`），并在执行时强制遵守。

#### Scenario: Policy requires docker sandbox
- **GIVEN** policy 为 `bash` 或 `run_command` 指定 `sandbox_mode=docker`
- **WHEN** LLM 尝试调用该工具
- **THEN** 系统在 Docker sandbox 中执行命令（workspace 挂载、默认无网络）
- **AND** tool output 标注该次执行的 sandbox_mode

#### Scenario: Sandbox unavailable is actionable failure
- **GIVEN** policy 要求 `sandbox_mode=docker` 但运行环境不可用（Docker 未安装或不可用）
- **WHEN** LLM 调用该工具
- **THEN** 系统拒绝执行并返回可操作错误（包含如何安装/开启或如何降级策略）

### Requirement: Global kill-switch MUST override all policies
系统必须 (MUST) 保留 `ONEAGENT_DISABLE_TOOL_*` 作为全局 kill-switch，优先级最高。

#### Scenario: Kill-switch disables tool globally
- **GIVEN** 设置 `ONEAGENT_DISABLE_TOOL_BASH=1`
- **WHEN** 任意 principal 请求使用 `bash`
- **THEN** 系统拒绝并返回“全局禁用”原因

### Requirement: Tools MUST declare mutability + reversibility for safety governance
系统必须 (MUST) 为每个可执行工具维护一份“安全元信息”（registry 或等价机制），至少包含：
- `effect=read_only|mutating`（是否会修改持久资产/外部状态）
- `reversibility=rollbackable|irreversible`（是否具备回退/补偿路径；不要求一定是“完美回滚”，但必须可操作）

当工具缺少上述元信息（例如用户自定义 tool/MCP tool 未声明）时，系统必须 (MUST) 采用保守默认值（例如视为 `mutating+irreversible`）并触发更严格的策略/审批要求，而不是 silent allow。

#### Scenario: Unknown tool is treated as high-risk by default
- **GIVEN** 系统挂载了一个未声明安全元信息的自定义 tool
- **WHEN** 系统为该 principal 生成可用工具列表
- **THEN** 系统默认将该 tool 视为高风险（例如 `mutating+irreversible`）
- **AND** 若策略未显式允许高风险工具，则该 tool 不应被暴露给 LLM

### Requirement: Policy MUST support approval gating for high-risk tool executions
系统必须 (MUST) 支持在 tool permissions policy 中为工具定义“审批约束”（例如 `approval=required|auto` 或等价约束），并在**执行阶段**强制执行。

当某次 tool call 需要审批时，系统必须 (MUST)：
- 在未获得批准前不执行该 tool call（fail-closed）
- 返回可操作的“待审批”响应（包含 tool_id、参数摘要、风险提示、以及回退/不可逆说明）
- 将审批决策（approve/deny）与理由写入审计证据链（trace/receipt）

#### Scenario: Tool call requires approval and is blocked until approved
- **GIVEN** policy 对某 mutating tool 设置 `approval=required`
- **WHEN** LLM 触发该 tool call
- **THEN** 系统不执行该 tool call
- **AND** 返回 “approval_required” 的可操作响应（包含风险与回退信息）
- **WHEN** 用户批准该次调用
- **THEN** 系统才执行该 tool call 并返回结果

### Requirement: Approval grants MUST be scoped, non-replayable, and auditable
系统必须 (MUST) 将一次审批授权绑定到明确的“调用指纹”（例如 `attempt_id + tool_id + canonical_args_hash`），并确保：
- 该授权不可被跨 attempt 复用（避免 replay）
- 该授权不可被参数漂移复用（相同 tool 不同参数必须重新审批）
- 审批记录可被审计与回放（包含批准者、时间、风险提示版本、以及执行结果引用）

#### Scenario: Approval cannot be replayed across attempts
- **GIVEN** attempt A 中用户批准了一次 tool call（tool_id 相同）
- **WHEN** attempt B 中 LLM 触发“相同 tool 但不同 attempt_id”的调用
- **THEN** 系统不得复用 attempt A 的审批结果
- **AND** 必须再次进入审批或按策略拒绝

### Requirement: Tool permissions UI MUST clearly show the current principal context
系统必须 (MUST) 在工具权限治理页面清晰标识当前正在编辑/查看的 `principal_id` 以及该 principal 的生效策略快照信息（best-effort）。

#### Scenario: Principal context is visible
- **GIVEN** 用户在工具权限页面查看策略
- **WHEN** 页面渲染完成
- **THEN** 页面清晰展示当前 `principal_id` 与 policy snapshot 基本信息（best-effort）

### Requirement: Tool policy editor MUST be usable for real JSON
系统必须 (MUST) 提供一个可用的策略编辑体验：足够高度、等宽字体、格式化/校验入口（best-effort）；避免“迷你 textarea”导致不可用。

#### Scenario: Policy editor supports formatting and comfortable editing
- **GIVEN** 用户编辑 tool policy JSON
- **WHEN** 用户输入/粘贴策略内容
- **THEN** 编辑器区域具备足够高度且可一键格式化（best-effort）

