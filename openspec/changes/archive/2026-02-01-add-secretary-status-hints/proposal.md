# Change: Add secretary-mode status hints (low-noise)

## Why
在 secretary-first 体验下，Sidebar/工作台入口默认被隐藏；这符合“像微信聊天”的心智，但也带来一个副作用：
用户在秘书模式里看不到关键系统状态（例如 SOP 待治理数量），容易错过“可以收割/可以治理”的时机。

我们需要一个 **低噪声** 的方式，在不恢复“管理系统外观”的前提下，把关键状态提示给用户，并提供“一键进入完全模式处理”的路径。

## What Changes
- 在秘书模式下，在 Chat header 等低噪声位置展示关键状态提示（先做 SOP 待治理数量；后续可扩展任务运行中等）。
- 点击提示会自动切换到完全模式并跳转到对应页面（例如 SOP 治理工作台）。
- 抽取可复用的状态拉取逻辑（composable），供 Sidebar 与 secretary hints 复用，避免重复定时器/重复实现。

## Impact
- Affected specs:
  - `chat-ux`
- Affected code (expected):
  - `frontend/src/components/ChatBox.vue`
  - `frontend/src/components/Sidebar.vue`
  - `frontend/src/components/SecretaryStatusHints.vue` (new)
  - `frontend/src/composables/useLedgerStatusToday.ts` (new)

