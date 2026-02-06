# system-mcp-server Specification

## Purpose
Defines the MCP server interface that exposes oneAgent runtime resources and events, enforcing local-only defaults, principal auth/policy, and traceable calls.

## Requirements
### Requirement: MCP server MUST be local-only by default
系统必须 (MUST) 提供一个 MCP server，并默认仅允许本机访问（local-only），以降低暴露面与误用风险。

系统必须 (MUST) 明确声明其监听方式（例如 loopback HTTP 或 stdio），并提供可配置项用于显式开启远程访问（默认关闭）。

#### Scenario: Default configuration rejects non-local connections
- **GIVEN** MCP server 使用默认配置启动
- **WHEN** 一个非本机来源尝试连接 MCP server
- **THEN** 连接被拒绝（或等价安全行为）

### Requirement: MCP server MUST enforce principal auth and policy
系统必须 (MUST) 对 MCP server 的每一次请求执行身份与权限校验：
- 必须 (MUST) 关联到一个 `principal_id`（显式 token 或等价机制）
- 必须 (MUST) 复用 `system-tool-permissions` 的策略体系，不得绕过
- 未授权请求必须 (MUST) 返回可操作错误（best-effort）

#### Scenario: Unauthorized MCP request is rejected
- **GIVEN** 客户端未提供有效身份凭证
- **WHEN** 客户户端请求读取 task 列表
- **THEN** MCP server 拒绝并返回未授权错误（best-effort）

### Requirement: MCP server MUST expose read-only resources for core runtime
系统必须 (MUST) 至少暴露以下 read-only MCP resources（或等价工具）：
- tasks 列表（含 workspace、状态、attempts 概览）
- 单个 task/attempt 详情（含 artifacts 指针）
- receipts 列表与单个 receipt 详情（含 artifacts 指针）
- skills 列表（用于发现/治理入口）

这些 read-only 资源必须 (MUST) 不产生副作用（不得创建 task、不得触发学习管线）。

#### Scenario: Listing tasks via MCP has no side effects
- **GIVEN** 系统存在若干 tasks
- **WHEN** 客户端通过 MCP 请求 tasks 列表
- **THEN** 返回 tasks 列表数据
- **AND** 系统不创建新 task、不修改 task 状态（除非已有后台自然推进）

### Requirement: MCP server MUST provide an event stream for tasks (best-effort)
系统必须 (MUST) 提供一个 MCP 事件流（或等价机制）用于订阅 task/attempt 的状态变化与关键 events（best-effort），以便外部客户端实现通知/日报与“挂机收割”体验。

#### Scenario: Client subscribes and receives attempt status changes
- **GIVEN** 客户端已订阅 task events
- **WHEN** 某 task attempt 状态从 `queued` 变为 `running`
- **THEN** 客户端收到一条对应事件（best-effort）

### Requirement: MCP server calls MUST be traceable
系统必须 (MUST) 将 MCP server 的调用纳入可追溯证据链（best-effort）：
- 记录调用的时间、principal、方法名、参数摘要、结果摘要
- 可引用到对应的 task/receipt（如果相关）

#### Scenario: MCP call is recorded in trace
- **WHEN** 客户端通过 MCP 读取某个 task 详情
- **THEN** 系统记录一条 trace/evidence 条目（best-effort）
