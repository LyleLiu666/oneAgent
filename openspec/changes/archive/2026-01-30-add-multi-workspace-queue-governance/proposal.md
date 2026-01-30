# Change: Add multi-workspace queue governance (policy, resource limits, scheduling)

## Why
当前 `system-task-queue` 的默认策略是：同一 workspace 串行，不同 workspace 并行且不设固定上限。这对“尽快完成”很友好，但在真实使用中会带来工业化问题：
- 多 workspace 同时 running 容易把本机 CPU/内存/磁盘打满，导致整体变慢甚至卡死
- 缺少“暂停/限速/优先级”会让用户很难在多个工作线之间做资源分配
- 缺少“日程化/定时入队”会阻碍把 oneAgent 当成长期自动化系统

我们需要在不牺牲默认顺滑度的前提下，引入**可选的**队列治理与资源策略。

## What Changes
- 扩展 `system-task-queue`：
  - 全局/每 workspace 的并发与资源策略（默认不限制；用户可配置）（best-effort）
  - queue controls：pause/resume、priority、fairness（best-effort）
  - schedules：定时入队（best-effort）

## Impact
- Affected specs: `system-task-queue`
- Affected code (expected): `backend/internal/taskqueue/*`, `backend/internal/handler/*`, `frontend/src/components/TaskQueuePanel.vue`
- Related docs: `docs/roadmap-longterm.md`

