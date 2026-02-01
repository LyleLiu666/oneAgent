# Change: Add secretary-mode deliverable cards for task artifacts (low-noise)

## Why
秘书模式的目标是“像微信一样只聊天”，但长任务会在 Task Queue 中异步完成。当前秘书模式下用户能看到“任务运行中”的低噪声提示，也能把输入 handoff 成后台任务，但**缺少任务完成后的“交付”入口**：
- 用户需要切回完全模式进入任务工作台，才能查看 findings/diff/test report 等产物
- 这让“微信心智”被迫回到“管理系统心智”，削弱秘书模式的闭环体验

## What Changes
- 在 **秘书模式** 下展示“交付卡片”（deliverable cards），覆盖 **仅任务产物**：
  - 从 `GET /api/tasks` 拉取已完成的任务 attempt（终态）并提取其 artifacts（findings/diff/test_report/trace 等，best-effort）
  - 以低噪声卡片形式呈现：标题/状态/简短 summary + 可点击的 artifact 入口
  - 点击 artifact 在秘书模式内预览内容（通过 `/api/tasks/:id/attempts/:attempt_id/artifacts/:kind`），无需切到完全模式
- 控制噪声：只展示最近 N 个完成任务（例如 3 个，best-effort），并对空 artifact 做 best-effort 隐藏/降级。

## Impact
- Affected specs:
  - `chat-ux`
- Affected code (expected):
  - `frontend/src/components/ChatBox.vue`（挂载区）
  - `frontend/src/components/SecretaryTaskDeliverables.vue`（NEW）
  - `frontend/src/components/SecretaryTaskDeliverables.test.ts`（NEW）
  - `frontend/src/api/client.ts`（复用既有 `getTaskAttemptArtifact` 等，无需改动）

