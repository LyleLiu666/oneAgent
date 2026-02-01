## ADDED Requirements

### Requirement: Chat streaming MUST be resilient to reload (best-effort)
当用户刷新页面或网络短暂断开导致 SSE 中断时，系统必须 (MUST) 确保本次回复生成不会因此被动停止（best-effort）；系统应该 (SHOULD) 允许用户重新打开页面后继续观察该 session 的流式输出（best-effort）。

#### Scenario: Refresh does not stop generation
- **GIVEN** 用户在 session S 中触发了一次 assistant 回复生成（处于流式返回中）
- **WHEN** 用户刷新页面导致 SSE 连接断开
- **THEN** 后端生成过程不应因为 SSE 断开而被动停止（best-effort）
- **AND** 该回复最终应落盘到该 session 的历史消息中（best-effort）

#### Scenario: Client can re-attach after reload (best-effort)
- **GIVEN** session S 存在进行中的流式输出
- **WHEN** 用户刷新页面后重新进入 Chat UI
- **THEN** UI 应自动尝试重新 attach 到 session S 的进行中 stream（best-effort）
- **AND** UI 应继续展示该回复的流式输出（best-effort）

### Requirement: Chat UI MUST provide an explicit Stop button that discards the current reply
当 assistant 正在生成回复时，Chat UI 必须 (MUST) 提供可发现的“停止”按钮；停止不应依赖刷新页面或网络波动等副作用。

当用户主动停止时，系统应将本次 assistant 回复视为被丢弃（discard；best-effort）：不应写入会话历史。

#### Scenario: Stop cancels generation and discards assistant reply
- **GIVEN** assistant 正在为当前 session 生成回复（流式中）
- **WHEN** 用户点击 “停止”
- **THEN** 前端向后端发起 stop/cancel 请求（best-effort）
- **AND** 后端停止进一步生成（best-effort）
- **AND** 系统不应将本次 assistant 回复写入会话历史（discard；best-effort）
- **AND** UI 进入非流式状态，允许用户继续发送新消息（best-effort）

#### Scenario: Stop affects all viewers of the same session
- **GIVEN** 同一 session 在多个浏览器标签页/窗口中被同时打开且正在生成回复
- **WHEN** 任意一个页面点击 “停止”
- **THEN** 该 session 的本次回复生成应被停止（best-effort）
- **AND** 其它页面的流式输出也应尽快停止并进入非流式状态（best-effort）
