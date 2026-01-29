# Change: Add execution safety invariants (approval + rollback + non-bypassable scripts)

## Why
oneAgent 的核心价值是“在用户电脑上完成真实工作”。但只要 agent 具备工具与脚本执行能力，就会面对两个结构性风险：
- **不可控的副作用**：没有审批与回退，任何一次错误调用都可能造成不可逆损失，agent 很难从 demo 走向可用。
- **脚本绕过**：即便文件工具有 `file_scope` 等约束，若脚本/命令在宿主进程权限下执行，依然可能绕过策略直接修改系统或用户资产。

本变更把“安全”从实现细节提升为 **愿景级不变量（invariants）**：默认安全、显式提权、全程可审计、尽量可回退。

## What Changes
- 引入并固化三条执行安全不变量：
  - **审批不变量**：高风险/不可逆工具调用必须进入审批流程（单用户场景即本地确认），且审批证据进入 trace/receipt。
  - **回退不变量**：所有可修改资产的工具调用必须具备回退路径（tool rollback / 系统 checkpoint / 明确的补偿机制三选一）；不满足者默认不得暴露给 agent。
  - **不可绕过不变量**：脚本/命令执行必须遵循策略边界；当缺少“硬边界执行后端”时，命令类工具必须退化为只读（避免通过脚本绕过文件工具约束）。
- 允许用户换取自由，但不允许静默突破边界：
  - **沙箱/隔离执行后端是可选项**（例如 docker），但当用户希望启用“可写命令执行”等高自由能力时，系统必须要求硬边界后端；否则返回可操作错误或退化为只读。
  - **MCP/用户自定义 tool**：业务语义安全由提供方负责，但 oneAgent 必须兜底“能力边界不被绕过”（统一 policy/approval/rollback/audit 管道）。

## Impact
- Affected specs: `system-tool-permissions`, `local-runtime`, `system-task-queue`
- Affected code (expected):
  - Backend: `backend/internal/tool/*`, `backend/internal/runtime/*`, `backend/internal/taskqueue/*`, `backend/internal/workledger/*`
  - Frontend: high-risk tool 的审批 UI（最小可用即可）
- Compatibility:
  - 默认行为保持“安全优先”：无硬边界后端时，命令类工具默认只读；需要更高权限时必须显式开启并可审计。

## Open Questions
- “硬边界执行后端”优先支持哪些实现：Docker、macOS sandbox、Windows Job Object、Linux namespaces/bwrap？
- “可回退资产”的范围定义：仅 workspace 资产 vs. 用户显式允许的额外目录；以及大文件/大目录的快照成本策略。
- 不可逆工具（如网络侧写入）的治理：是否一律不提供，还是通过“显式不可逆确认 + 补偿 hooks（若有）”启用？

