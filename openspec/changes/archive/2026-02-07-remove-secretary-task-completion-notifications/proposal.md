# Change: Remove secretary-mode task completion chat notifications

## Why
秘书模式的目标是“低噪声 + 自然语言对话”。当前在后台任务完成/失败时自动向对话区追加“后台任务已结束/交付已更新”等回执，会在重试/多任务场景下造成刷屏，污染对话心智，用户也明确表示不想看到这些通知。

我们希望把“任务状态/交付物查看”收敛到任务面板（交付/排障）里：**聊天只承载自然语言对话与必要的回执**，避免系统过程性提示占用主对话区。

## What Changes
- 秘书模式下：后台任务进入终态时不再向 chat message list 注入任何 `assistant text` 通知
- 保留任务面板（交付/排障）作为查看任务状态与交付物的入口
- 对已落盘的历史本地通知（secretary local messages）做 best-effort 过滤与清理，升级后不回放刷屏回执

## Impact
- Affected specs:
  - `openspec/specs/chat-ux/spec.md`
- Affected code:
  - `frontend/src/components/ChatBox.vue`
  - `frontend/src/components/SecretaryChatBox.vue`
  - `frontend/src/lib/secretaryLocalMessages.ts`
- Tests:
  - Update ChatBox/SecretaryChatBox unit tests to assert no chat injection on `task-completed`
  - Add regression test for filtering legacy local notifications

