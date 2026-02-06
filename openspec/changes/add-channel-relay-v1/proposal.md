# Change: Add channel relay v1 for inbound delegation and outbound delivery notifications

## Why
平台若只停留在本地 UI，入口覆盖会受限。需要最小渠道中继能力，把外部消息安全接入秘书，并把任务完成通知回推到来源渠道。

## What Changes
- 新增 channel relay 能力（v1）：
  - 入站：渠道消息 -> secretary inbox
  - 出站：任务终态通知 -> 渠道线程
- 建立渠道消息幂等处理（provider message id 去重）
- 建立渠道线程与 secretary session/task 的可追溯绑定
- 保持默认 local-first 与权限边界（不绕过 auth/policy）

## Impact
- Affected specs:
  - `system-channel-relay` (new)
  - `system-secretary-orchestration`
- Affected code:
  - channel ingress webhook adapter
  - outbound notifier
  - channel-session mapping store
