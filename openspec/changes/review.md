# OpenSpec Changes: Priority & Progress

更新时间：2026-01-29

本文件只记录**当前 active changes** 的优先级与进度快照；历史内容不再维护。
已完成变更请看 `openspec/changes/archive/`，真实进度以 `openspec list` 为准。

## Priority（高 → 低）

### P0（正在推进 / 近期必须收敛）
- `add-worktree-attempt-isolation`（8/8）：git worktree 隔离 attempt（执行根目录 + 生命周期管理）
- `add-mcp-server`（7/7）：对外暴露 MCP server（local-only + auth/policy + events）

### P1（强 UX 价值 / 可并行）
- `update-task-events-waterfall`（10/10）：事件/日志瀑布（Pretty/Raw + filter/search + no-flicker refresh + tail artifacts）
- `update-ux-error-surface`（11/11）：统一错误披露（安全/可追踪/可操作）+ 逐页修 UX

### P2（低成本高收益 / 穿插）
- `fix-skill-read-not-found-ux`（4/7）：skill.read not-found 的可行动 UX

### P3（体验线 / 不阻塞主线）
- `add-secretary-mode-chat`（0/5）：提供“秘书模式”纯聊天体验（独立路由 + 低噪声）

## Snapshot（来自 `openspec list`）
- `update-task-events-waterfall`：10/10 tasks
- `update-ux-error-surface`：11/11 tasks
- `add-secretary-mode-chat`：0/5 tasks
- `add-mcp-server`：7/7 tasks
- `add-worktree-attempt-isolation`：8/8 tasks
- `fix-skill-read-not-found-ux`：4/7 tasks
