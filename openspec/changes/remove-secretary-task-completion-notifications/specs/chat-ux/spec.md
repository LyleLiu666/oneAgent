# chat-ux Spec Delta

## MODIFIED Requirements

### Requirement: Secretary mode MUST NOT inject task completion notifications into chat (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下，避免在后台任务进入终态（完成/失败）时自动向 chat message list 追加过程性 `assistant text` 通知（best-effort）。任务状态与交付入口应集中在任务面板/交付卡片中呈现（best-effort），以保持主对话区为自然语言对话。

#### Scenario: Task completion does not append chat messages (best-effort)
- **GIVEN** 某任务 latest attempt 从 `queued/running` 进入终态（best-effort）
- **WHEN** UI 在下一次轮询/刷新中观察到该状态跃迁（best-effort）
- **THEN** chat message list 不追加新的 `assistant text` 消息（best-effort）
- **AND** 用户可在任务面板/交付卡片中查看交付物入口（best-effort）

### Requirement: Secretary local messages MUST filter legacy completion receipts (best-effort)
系统必须 (MUST) 对历史已落盘的“任务完成/失败回执模板”做 best-effort 过滤，避免升级后回放刷屏（best-effort）。过滤仅针对旧版固定模板，不影响用户手动输入或正常对话回执（best-effort）。

#### Scenario: Legacy receipts are hidden and cleaned up (best-effort)
- **GIVEN** 本地存储存在旧版任务完成回执（best-effort）
- **WHEN** UI 加载 secretary local messages（best-effort）
- **THEN** 这些旧回执不被渲染到对话区（best-effort）
- **AND** 系统 best-effort 清理本地存储中对应条目（best-effort）

