## Context
当前后端错误多以 `{"error": err.Error()}` 的形式直出，前端多页面直接渲染该字符串。它同时带来：
- 安全/隐私风险：可能包含绝对路径、内部策略/实现细节、甚至不应暴露的上下文片段。
- 可用性风险：错误不可操作（用户不知道“下一步怎么办”），且页面间呈现不一致。

此外，`docs/ux_critique.md` 指出多个页面存在空状态缺失、主操作不突出、信息密度低、编辑器/筛选区可用性差等问题。

## Goals / Non-Goals
### Goals
- 统一错误响应契约与前端展示方式：**默认安全、可操作、可追踪**。
- 将 UX critique 抽象为可验收的 requirements（场景化），并明确“先做什么、后做什么”。
- 保持本地开发/排障能力：提供 `request_id`（或等价）用于定位 trace/receipt。

### Non-Goals
- 不做一次性重写所有页面/引入大型 design system。
- 不在本变更中调整 tool permission 的具体 allowlist（例如 readonly 是否允许 `awk/sed`），仅保证错误披露与可操作性。

## Decisions
### Decision: Error disclosure must be layered
- **Public error surface（默认）**：面向 UI/HTTP API 的 `error` 文案必须安全、简短、可操作；不得包含内部栈/实现细节与敏感信息。
- **Debug details（显式）**：原始错误与上下文保留在 trace/log/receipt 中；UI 仅在显式操作下展示更多信息，并以 `request_id` 作为关联键。

### Decision: Error response remains backward compatible
错误响应保持 `error: string` 以避免破坏现有前端；新增字段用于更强的可观测性与 UX：
- `code`: 稳定的错误码（用于 UI 分流/提示）
- `hint`: 可选的下一步建议
- `request_id`: 请求关联 ID（用于 trace/日志定位）

### Decision: UX improvements are incremental, page-scoped
按页面落地“空状态/信息密度/主操作/编辑器可用性”的最小改动集，并用 OpenSpec scenarios 验收，避免无边界的“整体重构”。

## Alternatives considered
- 仅前端做“错误文案替换”：无法解决后端直出敏感信息的问题，且仍缺少可追踪性。
- 后端统一返回“Internal Server Error”：安全但不可用，用户无从行动。

## Risks / Trade-offs
- 统一错误映射需要覆盖面：先从 `backend/internal/handler/*` 的 HTTP 入口做起，再向内部工具/任务管线扩展。
- 多语言问题：项目默认中文；错误码稳定，`error/hint` 默认中文，后续可国际化。

## Migration Plan
1) 引入 `request_id` 生成与透传（header + JSON 字段）。
2) 新增后端统一的 error responder，逐步替换 `gin.H{"error": err.Error()}`。
3) 前端引入统一错误组件与解析函数，逐步替换页面内联 `e.data.error`。
4) 页面级 UX 逐步落地（先 /tasks 与 /governance/tools，再扩展到其它页面）。

## Open Questions
- Debug details 的展示开关：是否需要 `?debug=1` 或仅在本地/管理员 principal 下可见？
- `request_id` 与 task attempt trace 的映射：仅请求级，还是也支持 “attempt_id/trace_id” 一键跳转？

