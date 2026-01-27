# Change: Add in-app task notifications

## Why
oneAgent 目标是“长时自主交付”，用户不可能一直盯着屏幕。即使暂不做 webhook/IM 通知，产品也需要在 UI 内提供清晰的“任务完成/失败/可续跑”的提示与可见性。

## What Changes
- 在 task workbench 与 chat 内 task panel 中增加 **in-app notifications**：
  - 轮询刷新任务列表时，检测任务状态从 `queued/running` 进入终态（`succeeded/failed/timed_out/interrupted/canceled`）
  - 在 UI 中展示“更新列表/徽标”，支持一键清空
  - 默认首次加载仅建立基线，不对历史任务弹出通知（避免刷屏）

## Impact
- Affected specs: `local-runtime`
- Affected code: `frontend/src/views/TaskWorkbench.vue`, `frontend/src/components/TaskQueuePanel.vue`, `frontend/src/lib/*`

