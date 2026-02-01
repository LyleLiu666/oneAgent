# Change: Update secretary task handoff UX (suggest on send + chat receipt)

## Why
当前秘书模式虽然支持“入队任务”与“交付/恢复”，但仍有两个体验断点：

1) **入口不够像微信**：用户需要点击一个额外按钮才能把事情交给后台；而在微信心智里，用户只会“发消息”，秘书应当接住并引导。
2) **缺少对话留痕**：handoff 成功后不会在对话区留下“我说了什么 / 系统做了什么”的记录，容易让用户产生“我刚刚发出去了吗？”的不确定感。

## What Changes
- 在 **秘书模式** 下，当用户点击 Send 且消息看起来是“长任务/需要交付物”（best-effort）时，弹出一个低噪声确认：
  - “建议交给后台任务执行（可断点恢复/会产出交付）”
  - 选项：`交给后台` / `作为聊天发送`
- 当 handoff 创建任务成功后，在对话区追加一条用户消息 + 一条助手回执（best-effort），强化“微信心智”的确定性。

## Impact
- Affected specs:
  - `chat-ux`
- Affected code (expected):
  - `frontend/src/components/ChatBox.vue`
  - `frontend/src/components/ChatBox.test.ts`

