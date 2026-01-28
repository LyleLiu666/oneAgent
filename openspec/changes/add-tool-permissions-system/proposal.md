# Change: Add per-user tool permissions system

## Why
当前 tool 权限是“最小可交付”：`ONEAGENT_DISABLE_TOOL_*` + bash 的 `ONEAGENT_BASH_ALLOW_RM` 逻辑。这带来几个问题：
- **不是 per-user/per-session**：无法对不同用户设置不同的工具组合与约束。
- **保护点过浅且易绕过**：单纯限制 `rm` 这类 token，仍可能通过脚本/解释器/间接手段进行破坏性操作。
- **缺少统一的审计与可解释性**：拒绝原因、权限来源与匹配规则难以追溯，无法做长期治理。
- **难以与 task queue / subagent 组合**：后台任务与子 Agent 需要明确的权限传播与收敛边界。

## What Changes
- 新增 `system-tool-permissions` 规格：定义 principal/role/policy/rule、决策顺序、审计与可解释错误。
- 将 tool 权限从“进程级 env 开关”升级为“按用户/会话/任务”的策略系统（仍保留 `ONEAGENT_DISABLE_TOOL_*` 作为 break-glass kill switch）。
- 移除 `ONEAGENT_BASH_ALLOW_RM`（不再以单一命令 token 作为权限模型）。
- 为 `bash`/`run_command` 引入可配置的命令 allowlist/profile，降低“写脚本绕过”的风险。
- 在 TaskQueue/Subagent 中传播并收敛权限：子 Agent/后台 task 不得获得比父上下文更高的权限；权限变更通过新 attempt 生效。
- 提供最小管理面（API/CLI/配置）：可查看当前 principal 的生效权限与拒绝原因。

## Impact
- Affected specs: `local-runtime`, `auth-mode`, `workspace`, `system-file-tools`, `system-subagent-orchestration`, `system-task-queue`（新增 `system-tool-permissions`）。
- Affected code: `backend/internal/middleware/auth.go`, `backend/internal/tool/*`, `backend/internal/toolxml/*`, `backend/internal/taskqueue/*`, `frontend/*`（settings/workbench）。
- Migration: 默认单机体验保持不变；新增能力以向后兼容方式落地；`ONEAGENT_BASH_ALLOW_RM` 将被删除并在启动日志提示替代方案（策略系统）。
