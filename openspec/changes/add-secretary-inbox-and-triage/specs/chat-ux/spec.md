## ADDED Requirements

### Requirement: Secretary mode MUST support multi-message sending with quick acks and batched triage replies (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下支持“微信式连续发送”：用户可以连续发送多条消息，而不被“assistant 正在生成中”的状态阻塞；系统应该 (SHOULD) 在用户每次发送后尽快给出一个低噪声的快速确认（quick ack，**由 LLM 生成的短句**，best-effort）；系统应该 (SHOULD) 在短暂静默窗口后对“自上次归并以来的消息集合”输出一条低噪声的秘书式汇报（best-effort），而不是逐条进行完整分析回复。

#### Scenario: User can send multiple messages without being blocked
- **GIVEN** 用户处于秘书模式（best-effort）
- **WHEN** 用户在短时间内连续发送多条消息
- **THEN** UI 允许每条消息都被发送并显示在对话中（best-effort）
- **AND** 系统不会因为 assistant 正在生成而阻止用户继续发送（best-effort）

#### Scenario: Each message gets a quick ack before triage
- **GIVEN** 用户处于秘书模式（best-effort）
- **WHEN** 用户发送一条消息
- **THEN** UI 在短时间内展示一条 quick ack（由 LLM 生成的短句，best-effort）
- **AND** 该确认不要求等待后续 triage 汇总完成（best-effort）

#### Scenario: Quick ack is replayable after refresh (traceable)
- **GIVEN** 用户在秘书模式下发送了一条消息并收到 quick ack（best-effort）
- **WHEN** 用户刷新页面或重新进入会话（best-effort）
- **THEN** 对话中仍可看到同样的 quick ack 内容（best-effort）

#### Scenario: Secretary produces a single batched reply for a message burst
- **GIVEN** 用户处于秘书模式并连续发送了多条消息（best-effort）
- **WHEN** 用户停止输入并产生短暂静默窗口（best-effort）
- **THEN** 系统输出一条秘书式汇报，覆盖这批消息的归并理解（best-effort）
- **AND** 汇报可包含“正在推进的工作线/已派发的后台任务/需要用户确认的问题”（best-effort）
