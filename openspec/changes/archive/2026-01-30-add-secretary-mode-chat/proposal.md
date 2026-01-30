# Change: Add “Secretary Mode” (low-noise chat)

## Why
当前 Chat 作为“秘书入口”，仍混入了大量低频/诊断/治理信息（模型/工具选择、历史侧栏、trace/工具细节、任务面板等），对“只想和秘书对话、把任务委托出去”的用户来说噪声偏高。

参考 Moltbot 的产品形态：**对话通道是产品本体**，其余 UI 更像 control plane。oneAgent 也需要一个足够“低噪声”的纯聊天体验，与 Tasks/Governance/Ledger 等复杂交互窗口并存。

## What Changes
- Chat 增加“秘书模式”：
  - 仅保留：消息流 + 输入框 + 必要状态
  - 默认隐藏：历史侧栏、模型/工具选择、trace/工具细节、任务面板等低频内容
  - 允许一键切换回完整模式（不丢功能，只折叠复杂度）
- 提供独立路由用于直接进入秘书模式（例如 `/secretary`，best-effort）
- 高级能力不删除：完整模式仍可从全局菜单中进入（best-effort）
- 模式选择持久化（例如 localStorage），并支持通过 URL/入口快速进入（best-effort）

## Impact
- Affected specs: `chat-ux` (new)
- Affected code (expected): `frontend/src/components/ChatBox.vue`, `frontend/src/router/index.ts` (optional)
