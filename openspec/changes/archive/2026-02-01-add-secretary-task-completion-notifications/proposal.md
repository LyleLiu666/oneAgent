# Change: Add secretary-mode task completion notifications (low-noise)

## Why
在微信心智下，用户发出委托后不应反复“去看任务工作台”；秘书应当在后台任务完成/失败时主动告知，并把用户引导回“交付/下一步”，而不是暴露管理系统细节。

目前我们已经能：
- 在秘书模式创建任务（handoff）
- 在秘书模式看交付卡片（deliverables）
- 在秘书模式看到失败并一键恢复（recovery actions）

但缺少“像微信一样的主动消息”：任务完成后用户如果不看交付区，仍可能错过收割时机。

## What Changes
- 在秘书模式下，当某个任务的 latest attempt 从 `queued/running` 进入终态（`succeeded/failed/...`）时：
  - 在对话区追加一条低噪声助手通知（best-effort）
  - 通知只强调“结果已更新 + 去交付查看”，不展示任务治理细节
- 防刷屏：初次加载仅建立基线，不回放历史完成事件（best-effort）。

## Impact
- Affected specs:
  - `chat-ux`
- Affected code (expected):
  - `frontend/src/components/SecretaryTaskDeliverables.vue`（检测状态跃迁并 emit）
  - `frontend/src/components/SecretaryTaskDeliverables.test.ts`
  - `frontend/src/components/ChatBox.vue`（监听事件并追加消息）
  - `frontend/src/components/ChatBox.test.ts`（用 stub 验证监听行为）

