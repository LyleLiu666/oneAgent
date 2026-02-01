# Change: Add secretary-mode chat task handoff (manual-first)

## Why
秘书模式的目标是“像微信一样只聊天”，但真实使用中经常会出现需要长时间执行/可中断恢复/可交付留痕的工作（例如：跑测试、批量改代码、生成报告、整理证据链）。

当前后台任务（Task Queue）主要通过完整模式的工作台入口使用；在秘书模式下，用户不得不切回“管理系统外观”才能创建任务，破坏心智一致性。

## What Changes
- 在 **秘书模式** 的 Chat 输入区提供一个“交给后台/入队任务”的低噪声动作（manual-first）。
- 点击后将当前输入作为 `prompt` 创建后台任务（`POST /api/tasks`），并使用 best-effort 的 workspace 解析（优先使用已解析 workspace；否则尝试选择/兜底）。
- 成功后清空输入并提示“已交给后台”；失败时保留输入并展示可解释错误（best-effort）。
- 不自动切换到完全模式；进度与接管入口复用现有的 secretary status hints（例如任务 badge）。

## Impact
- Affected specs:
  - `chat-ux`
- Affected code (expected):
  - `frontend/src/components/ChatBox.vue`
  - `frontend/src/components/ChatBox.test.ts`
  - （可选）抽取小的 handoff 逻辑到 composable，避免继续膨胀 `ChatBox.vue`

