## ADDED Requirements

### Requirement: 系统必须提供可组合的 tool 权限策略（principal/role/policy）
系统必须 (MUST) 支持以 `principal_id`（用户/主体）为隔离边界的 tool 权限控制，并允许通过 role/policy 复用权限集合。

系统必须 (MUST) 支持至少两类策略效果：
- `allow`：允许使用某个 tool（可带约束）
- `deny`：拒绝使用某个 tool（可带约束）

#### Scenario: 不同 principal 的工具集合不同
- **GIVEN** principal `alice` 的 policy 拒绝 `bash`
- **AND** principal `bob` 的 policy 允许 `bash`
- **WHEN** 系统为 `alice` 构建 LLM tool list
- **THEN** tool list 中不包含 `bash`
- **WHEN** 系统为 `bob` 构建 LLM tool list
- **THEN** tool list 中包含 `bash`

### Requirement: 工具权限必须同时约束“暴露给 LLM”与“实际执行”
系统必须 (MUST) 在两个层面应用同一套权限决策：
1) tool 暴露（LLM tool list）
2) tool 执行（runtime enforcement）

系统必须 (MUST) 在执行时再次校验权限；不得 (MUST NOT) 仅依赖“未暴露即不可调用”的假设（LLM 可能缓存/猜测 tool name）。

#### Scenario: 未暴露的 tool 被调用时仍会被拒绝
- **GIVEN** principal 的 policy 拒绝 `bash`
- **WHEN** LLM 仍尝试调用 `bash`
- **THEN** 系统拒绝该调用并返回明确错误（包含 `tool_id=bash` 与拒绝原因）

### Requirement: 权限策略必须支持约束（constraints）与可解释拒绝
系统必须 (MUST) 支持在规则上附加至少以下约束，并在拒绝时返回可解释原因：
- 文件写入范围约束（glob，基于 `<workspace>/` 的相对路径）
- 绝对路径读取开关（是否允许读取 `<workspace>/` 外的绝对路径）
- 命令工具 profile（例如 `readonly|dev|full`；至少支持 allowlist 模式）

#### Scenario: 文件写入范围约束生效
- **GIVEN** principal 的 policy 仅允许写入 `backend/**`
- **WHEN** principal 尝试写入 `frontend/App.vue`
- **THEN** 系统拒绝该写入并返回“path is outside policy scope”的可理解错误

### Requirement: Break-glass 全局禁用必须覆盖策略系统
系统必须 (MUST) 支持通过环境变量全局禁用指定 tool，并且该禁用必须 (MUST) 覆盖所有 principal 的策略（最高优先级）。

#### Scenario: 环境变量禁用覆盖 allow policy
- **GIVEN** principal 的 policy 允许 `bash`
- **AND** 环境变量设置 `ONEAGENT_DISABLE_TOOL_BASH=1`
- **WHEN** principal 尝试调用 `bash`
- **THEN** 系统拒绝并返回明确错误（包含禁用来源为环境变量）

### Requirement: 系统必须记录权限决策与证据（审计）
系统必须 (MUST) 为每次 tool 调用记录权限决策元信息（allow/deny、principal_id、tool_id、命中的规则/约束摘要），并将其写入 trace（或等价的可追溯日志）。

#### Scenario: trace 可定位到拒绝原因
- **GIVEN** 某次 tool 调用因权限被拒绝
- **WHEN** 用户查看该次会话/task 的 trace 日志
- **THEN** trace 中包含该次拒绝的 principal_id、tool_id 与规则摘要

