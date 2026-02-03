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
