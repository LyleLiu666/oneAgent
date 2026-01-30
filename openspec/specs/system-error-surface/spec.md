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

