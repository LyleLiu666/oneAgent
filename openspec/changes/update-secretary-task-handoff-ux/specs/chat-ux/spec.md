## ADDED Requirements

### Requirement: Secretary mode MUST suggest background task handoff for long-running requests (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下，对“明显是长任务/需要交付物”的消息给出低噪声引导（best-effort），优先建议交给后台任务队列执行，并允许用户选择“仍作为即时聊天发送”（best-effort）。

#### Scenario: Secretary suggests handoff on send for long tasks
- **GIVEN** 用户处于秘书模式（best-effort）
- **AND** 用户输入一条“看起来是长任务”的消息（best-effort）
- **WHEN** 用户点击 Send
- **THEN** 系统展示一个低噪声确认，让用户选择 `交给后台` 或 `作为聊天发送`（best-effort）

### Requirement: Secretary mode MUST provide a chat receipt after successful task handoff (best-effort)
系统必须 (MUST) 在秘书模式下，当用户将消息 handoff 成后台任务且创建成功时，在对话区追加一个“回执”以留痕（best-effort），至少包括：
- 用户消息（原始输入）
- 助手回执（确认已交给后台，交付物会出现在交付区）

#### Scenario: Handoff appends a receipt to the chat history
- **GIVEN** 用户处于秘书模式且 handoff 创建任务成功（best-effort）
- **WHEN** 系统完成创建
- **THEN** 对话区出现一条用户消息与一条助手回执（best-effort）
