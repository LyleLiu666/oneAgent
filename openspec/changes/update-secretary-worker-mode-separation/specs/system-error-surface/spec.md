# system-error-surface Spec Delta

## ADDED Requirements

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
