# Change: Add secretary-mode task status hints (low-noise)

## Why
在 secretary-first 体验下，用户默认只看到“像微信一样”的聊天界面；但后台任务（Task Queue）是一个核心能力：用户可能已经有任务在 `queued/running`，却在秘书模式里完全无感，错过“等待/接管/收割”的时机。

我们需要一个 **低噪声** 的方式，在不恢复“管理系统外观”的前提下，把关键任务运行状态提示给用户，并提供“一键进入完全模式处理”的路径。

## What Changes
- 在秘书模式下，在 Chat header 的 hints 区域展示任务队列的低噪声提示（例如运行中/排队中数量）。
- 点击提示会自动切换到完全模式并跳转到任务工作台（`/tasks`）。
- 抽取可复用的 task 状态拉取逻辑（composable），避免在多个组件中重复定时器与聚合逻辑。

## Impact
- Affected specs:
  - `chat-ux`
- Affected code (expected):
  - `frontend/src/components/SecretaryStatusHints.vue`
  - `frontend/src/composables/useTaskQueueStatus.ts` (new)
  - `frontend/src/components/SecretaryStatusHints.test.ts`
  - `frontend/src/components/ChatBox.test.ts` (mock additions)

