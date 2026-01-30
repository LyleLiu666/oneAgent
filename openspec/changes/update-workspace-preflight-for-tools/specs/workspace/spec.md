## ADDED Requirements

### Requirement: Tool-enabled sessions MUST preflight workspace requirements
当会话请求启用工具（例如 `tool_ids` 非空）且所启用工具依赖 workspaceRoot 作为默认作用域时，系统必须 (MUST) 在调用 LLM 前执行 workspace 前置校验：
- workspace 未设置 → fail-fast 返回可操作错误
- workspace 已设置 → 正常进入 LLM / tool loop（best-effort）

#### Scenario: Tools enabled but workspace missing fails fast
- **GIVEN** 用户发起聊天请求并启用 tools
- **AND** 当前会话未设置 workspaceRoot
- **WHEN** 系统准备调用 LLM
- **THEN** 系统直接返回“workspace 未设置”的可操作错误
- **AND** 系统不应调用 LLM（best-effort）
