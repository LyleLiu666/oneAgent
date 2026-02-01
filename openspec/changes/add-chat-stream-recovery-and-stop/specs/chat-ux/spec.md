## ADDED Requirements
### Requirement: Chat UI MUST allow stopping an in-flight assistant stream
When the assistant message is streaming, the UI MUST provide a visible stop control so users can end generation intentionally.

#### Scenario: Stop button is visible during streaming
- **GIVEN** assistant 消息处于 streaming 状态
- **WHEN** 用户查看对话界面
- **THEN** UI 必须展示 “Stop/停止生成” 控件（best-effort）

#### Scenario: After stop, the assistant message is finalized (best-effort)
- **GIVEN** assistant 消息处于 streaming 状态
- **WHEN** 用户点击 “Stop/停止生成”
- **THEN** UI 应停止追加内容并将该消息标记为已结束（best-effort）

### Requirement: System MUST provide clear feedback on stream interruptions
When the stream ends due to user stop or error, the UI MUST communicate the reason (best-effort) and MUST NOT silently hang.

#### Scenario: Stream ends due to user stop
- **GIVEN** 流式输出进行中
- **WHEN** 用户主动停止生成
- **THEN** UI 应显示“已停止/Stopped”（best-effort）

#### Scenario: Stream ends due to error
- **GIVEN** 流式输出进行中
- **WHEN** 后端返回 error 或流被中断
- **THEN** UI 应显示错误提示（best-effort）

