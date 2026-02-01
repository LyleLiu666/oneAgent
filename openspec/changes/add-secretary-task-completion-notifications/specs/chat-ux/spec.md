## ADDED Requirements

### Requirement: Secretary mode MUST notify in chat when a background task completes (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下，在后台任务完成/失败时以低噪声方式通知用户（best-effort），以强化“微信心智”的确定性，并减少用户去任务工作台查看的心智负担。

通知策略至少包括（best-effort）：
- 仅对“本次进入页面后发生的状态跃迁”提示（避免首次加载刷屏）
- 当任务 latest attempt 从 `queued/running` 进入终态时，追加一条助手消息提示“已完成/已失败 + 交付已更新”

#### Scenario: No history replay on first load
- **GIVEN** 用户进入秘书模式并加载任务列表
- **AND** 存在一些历史已完成任务
- **WHEN** 首次渲染完成
- **THEN** 系统不回放历史完成通知（best-effort）

#### Scenario: Chat notifies when a running task finishes
- **GIVEN** 某任务 latest attempt 处于 `running`
- **WHEN** 下一次轮询中该任务进入终态（例如 `succeeded` 或 `failed`）
- **THEN** 对话区追加一条助手通知（best-effort）

