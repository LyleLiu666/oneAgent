# Proposal: Secretary triage 必须把“待确认”说清楚并可追溯展示

## Why
当前秘书模式在 triage 需要用户确认时，可能只返回类似“有 N 个问题需要确认”的机械句式，但**没有把具体要确认什么展示出来**，导致用户不知道下一步怎么做、体验上像“卡死”。

这违背了“秘书 = 系统级中层管理者（Orchestrator）”的北极星体验：秘书应该用自然语言把关键信息讲清楚，并把执行放权给 worker，同时保证过程可追溯、可恢复。

## What Changes
- 当 triage 返回 `questions[]` 时：
  - 秘书对话中必须输出**可操作**的确认信息：逐条列出需要用户确认的具体问题，并说明“你只要回复什么/怎么选就能继续”（best-effort）。
  - UI 必须提供一个低噪声的“待确认”入口（例如 badge + 弹窗/面板）展示 `questions[]`，且在刷新/重进会话后仍可恢复显示（best-effort）。
  - 禁止只报“有 N 个问题需要确认”这种**仅计数、无内容**的回复（best-effort）。
- 责任边界澄清：
  - “可选项 + 默认值 + 风险提示 + 证据链接”的决策包装属于 **worker/Outcome Observer** 层（参见 `system-task-queue`），秘书层应以“放权 + 信任 + 可追溯”方式转达与组织，而不是在秘书层强行补齐/编造（best-effort）。

## Impact
- Affected specs:
  - `system-secretary-orchestration`
  - `chat-ux`
- Affected code (already landed): `backend/internal/secretary/orchestrator.go`, `frontend/src/components/ChatBox.vue` 等（实现先于本次补文档）。

## Non-goals
- 不改变 triage 的派工策略/算法（仅改善“待确认”的可见性与可操作性）
- 不引入新的持久化系统或新 API（优先复用现有 `GET /api/secretary/state` 与 triage 响应）
- 不把 worker 的“证据/风险/选项”包装搬到秘书层（维持分层职责）
