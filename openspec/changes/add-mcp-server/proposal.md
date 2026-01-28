# Change: Add MCP server to expose oneAgent runtime

## Why
oneAgent 不只是一个 UI，更是一个可编排的“本地 agent 运行时”：Task Queue、Work Ledger、Skills 等能力如果能通过行业标准协议暴露出来，就可以被外部客户端（例如桌面助手、快捷指令、其它 agents）调用，从而：
- 降低集成成本（不用为每个客户端写专有 HTTP SDK）
- 把 oneAgent 变成可复用的部门基础设施（可私有部署/托管执行）
- 为未来的“多 workspace 并行 + 队列 + 通知/日报”提供更通用的入口

vibe-kanban 的做法是提供本机 MCP server（local-only），将任务管理与项目运行时能力以 MCP tools/resources 暴露出去。oneAgent 可以参考其理念，但先从低风险（只读）能力开始。

## What Changes
- 增加一个 MCP server（local-only），将 oneAgent 的核心能力暴露为 MCP tools/resources：
  - Read-only：列出 tasks、读取 task/attempt 详情、读取 receipts/digest、读取 skill 列表（最小可用集）
  - Write (可选，受策略控制)：create task、cancel/resume task
- MCP server 必须复用现有的权限/身份体系（principal + policy），不得绕过 tool permissions
- MCP server 必须可观测：所有 MCP 调用都要进入 trace/evidence（best-effort）

## Impact
- Affected specs: `system-mcp-server`（new）
- Affected code (expected): `backend/internal/server/*`, `backend/internal/handler/*`, `backend/internal/auth/*`, `backend/internal/tool/*`

