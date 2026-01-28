## ADDED Requirements

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
系统必须 (MUST) 为 `bash`/`run_command` 提供可配置的命令 profile（`readonly/dev/full`），默认使用最小权限集合，并优先采用 allowlist。

#### Scenario: readonly profile blocks interpreters
- **GIVEN** 当前 profile 为 `readonly`
- **WHEN** 调用 `bash` 执行 `python`/`node` 等解释器
- **THEN** 系统拒绝该命令并返回可解释错误

### Requirement: Global kill-switch MUST override all policies
系统必须 (MUST) 保留 `ONEAGENT_DISABLE_TOOL_*` 作为全局 kill-switch，优先级最高。

#### Scenario: Kill-switch disables tool globally
- **GIVEN** 设置 `ONEAGENT_DISABLE_TOOL_BASH=1`
- **WHEN** 任意 principal 请求使用 `bash`
- **THEN** 系统拒绝并返回“全局禁用”原因

