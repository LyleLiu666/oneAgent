# Change: Add high-risk command approvals (auto/manual)

## Why
当前 `bash` / `run_command` 的审批能力是“按工具整体”生效的：要么全需要审批，要么全不需要。
这与真实使用的安全诉求不匹配：用户希望仅对**高风险命令**（删除/移动/安装/解释器执行等）进行审批与留痕，并且默认由“秘书”自动审批（降低摩擦），同时提供一个设置可切换为“用户手动审批”。

## What Changes
- 为命令类工具支持 `approval=high_risk` 语义：仅当本次 command 被判定为高风险时才进入审批门（poll/cancel 不触发）。
- 支持高风险审批的两种模式：
  - `auto`（默认）：系统自动批准并记录审批（保留审计痕迹），不中断执行。
  - `manual`：阻断执行并返回 `approval_required`，由用户在 UI 中 approve/deny。
- 增加用户级设置项与 UI 开关，用于切换 `auto/manual`（best-effort，默认 auto）。

## Impact
- Affected specs:
  - `system-tool-permissions`（命令工具的 high-risk 审批语义 + auto/manual 模式）
- Affected code (expected):
  - Backend: `backend/internal/tool/approval_gate.go`, `backend/internal/settingsdb/user_settings.go`, `backend/internal/handler/*`, `backend/internal/permissions/policy.go`
  - Frontend: `frontend/src/api/client.ts`, `frontend/src/views/Settings.vue`（或独立安全设置页）

