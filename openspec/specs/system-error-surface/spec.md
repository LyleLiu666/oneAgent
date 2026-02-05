# system-error-surface Specification

## Purpose
TBD - created by archiving change update-ux-error-surface. Update Purpose after archive.
## Requirements
### Requirement: HTTP/API errors MUST be safe, actionable, and traceable
系统必须 (MUST) 对所有对外 HTTP/API 错误返回“用户安全、可操作、可追踪”的结构：
- `error`：用户可读的简短错误文案（默认中文）
- `code`：稳定错误码（用于前端分流与测试断言）
- `request_id`：请求关联 ID（用于定位 trace/log）
- `hint`：可选的下一步建议（best-effort）

系统不得 (MUST NOT) 在默认错误文案中泄露以下内容：
- 内部实现细节（例如工具 profile 解析 token、内部 guard 名称）
- 敏感信息（例如 token、API key、隐私路径等）

#### Scenario: Tool permission denied is user-safe and traceable
- **GIVEN** 一次工具/命令执行因为策略被拒绝
- **WHEN** 系统以 HTTP/API 错误响应返回
- **THEN** 响应包含 `code` 与 `request_id`
- **AND** `error` 为用户安全文案（不包含原始内部错误串）
- **AND** 响应包含 best-effort `hint`（例如提示去“工具权限”页调整 policy/profile）

### Requirement: UI error presentation MUST use progressive disclosure
系统必须 (MUST) 在 UI 中统一错误展示方式：默认只展示 `error`（安全文案），并提供可发现的入口以显式展开更多信息（例如 `request_id`、调试线索）。

#### Scenario: Error banner hides technical details by default
- **GIVEN** 页面出现一次 API 错误
- **WHEN** UI 渲染错误提示
- **THEN** 默认仅展示安全文案
- **AND** 技术细节不默认显示（需要用户显式展开或复制 `request_id`）

### Requirement: Session/module mismatch MUST be surfaced with a stable error code (best-effort)
当客户端在错误的入口上操作了错误模块的 session（例如用 `/api/chat` 操作 `module=secretary` 会话，或用 `/api/secretary/*` 操作 `module=assistant` 会话），系统必须 (MUST) 返回可追溯、可恢复的错误（best-effort）：
- `code=session_module_mismatch`
- `error` 为用户安全中文提示（best-effort）
- `hint` 指向正确入口（例如“请切换到秘书模式/完整模式”或提供正确路由）（best-effort）
- `request_id` 用于定位 trace/log
建议 HTTP status 使用 `409 Conflict`（best-effort）。

#### Scenario: Chat API rejects non-assistant sessions
- **GIVEN** 客户端调用 `/api/chat` 且 session 实际 `module=secretary`
- **WHEN** 后端校验 session module
- **THEN** 返回 `code=session_module_mismatch`（best-effort）
- **AND** `hint` 提示改用秘书入口（best-effort）

### Requirement: Internal agent protocol/tool errors MUST be user-safe and actionable (best-effort)
当系统遇到“模型协议/结构化输出/工具参数”等工程类错误（例如 `invalid function arguments`、`invalid observer output (expected XML or JSON)`），系统必须 (MUST)：
- 优先在 agent loop 内自愈重试（best-effort）
- 若最终仍失败，返回用户安全的错误信息（不直接泄露原始 provider payload）（best-effort）
- 同时提供可追溯指针（`request_id` 或 trace/log path），便于定位（best-effort）

#### Scenario: Observer protocol error is not surfaced as "need user confirm"
- **GIVEN** 某次 attempt 在 outcome observer 阶段发生结构化输出解析错误（best-effort）
- **WHEN** 系统决定对外返回失败信息（达到重试上限后，best-effort）
- **THEN** 返回用户可理解的错误（best-effort）
- **AND** 不将其伪装成“需要用户确认才能继续”的空洞回执（best-effort）

