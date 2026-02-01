# Change: Add secretary-mode recovery actions for failed tasks (low-noise)

## Why
秘书模式的目标是“像微信一样只聊天”，但长任务会在后台执行。即便我们已经有：
- 任务运行中提示（Task badge）
- 手动 handoff 入队
- 任务产物交付卡片（deliverables）

仍存在一个关键断点：**任务失败时用户可能完全无感**（尤其是失败很快且未产出 artifacts 时），从而无法“继续/重试/排障”，破坏“可恢复默认体验”。

## What Changes
- 在 **秘书模式** 下，以低噪声方式提示“需要处理”的失败任务（best-effort），并提供下一步动作：
  - **继续**：调用 `POST /api/tasks/:id/resume` 重新入队（best-effort）
  - **排障**：一键切换到完全模式并进入任务工作台（best-effort）
  - （可选）若存在 trace/findings 等 artifacts，可直接预览（复用现有 artifact modal）
- 控制噪声：仅展示最近 N 个需要处理的任务（例如 2~3 个，best-effort）。

## Impact
- Affected specs:
  - `chat-ux`
- Affected code (expected):
  - `frontend/src/components/SecretaryTaskDeliverables.vue`（扩展 recovery section，复用 polling/preview modal）
  - `frontend/src/components/SecretaryTaskDeliverables.test.ts`

