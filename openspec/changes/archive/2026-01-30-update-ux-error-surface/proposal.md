# Change: Update UX + error disclosure surface (UI polish + safe errors)

## Why
当前 oneAgent 的 Web UI 存在两个会持续累积用户挫败感的问题：

1) **报错信息“直出”**：后端大量直接把 `err.Error()` 作为 `{"error": ...}` 返回，前端再把 `e.data.error` 直接渲染到页面。结果是：
- 用户看到内部实现细节/策略细节（例如 `command not allowed by profile=readonly: rm`），不够可操作且有“信息暴露”风险。
- 相同类型的错误在不同页面展示不一致（有时是 toast，有时是红框纯文本）。

2) **UX 信息密度与空状态体验差**：见 `docs/ux_critique.md`，多个页面存在空白死区、控件拥挤、主操作不突出、编辑器可用性差等问题，整体像“组件拼装的后台”，而不是“高效管理系统”。

## What Changes
- 定义并落地 **统一的错误响应契约**（保持兼容 `error: string`，新增 `code/request_id/hint` 等字段），并给出“公开错误 vs 调试细节”的披露分层。
- 后端：集中化错误映射与脱敏，默认只返回**用户安全且可操作**的错误信息；原始错误保留在 trace/log/receipt 证据链中。
- 前端：统一错误展示组件（默认只显示安全文案；显式展开后才展示更多信息/复制 `request_id`），并在关键页面应用。
- 结合 `docs/ux_critique.md`，为以下页面补齐可验收的 UX 要求（逐步实现，不做一次性大重构）：
  - Task Workbench（`/tasks`）：空状态、信息密度、workspace 选择器、状态反馈
  - SOP Governance（`/governance/sop`）：空状态、按钮显隐、文案降噪、布局
  - Skill Governance（`/governance/skills`）：名称可读性、一致性、编辑器空状态与保存按钮反馈
  - Tool Permissions（`/governance/tools`）：principal 可见性、工具列表可读性、策略编辑器可用性
  - Document Export（`/documents/export`）：主操作突出、工作区选择器语义更清晰、布局更聚焦
  - Ledger（`/ledger`）：列表摘要/截断、详情信息增量、筛选区可用性

## Impact
- Affected specs:
  - `system-error-surface` (new)
  - `system-task-queue`
  - `system-work-ledger`
  - `system-skill-management`
  - `system-tool-permissions`
  - `system-document-export`
  - `work-ledger-ux`
- Affected code (expected):
  - Backend: `backend/internal/handler/*`（错误返回）、可能新增通用 error helper/middleware
  - Frontend: `frontend/src/views/*`（统一错误展示 + 关键 UX 调整）、新增通用组件/工具函数
- Risks / trade-offs:
  - 过度“隐藏细节”会让排障变困难 → 通过 `request_id` 关联 trace，并提供“展开详情（显式）”
  - UX 改动较分散 → 通过 OpenSpec 的验收场景约束范围，按页面分步落地

## Notes
- 本变更仅创建 OpenSpec proposal；**未获评审批准前不进入实现阶段**。

