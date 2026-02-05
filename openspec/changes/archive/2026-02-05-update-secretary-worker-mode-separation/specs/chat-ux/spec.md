# chat-ux Spec Delta

## MODIFIED Requirements

### Requirement: Chat UI MUST provide “Secretary Mode” (low-noise)
系统必须 (MUST) 在 Chat UI 中提供一种“秘书模式”以降低默认信息噪声，并明确其语义为：**用户在秘书模式下与秘书对话（归并/解释/派工/进度汇报），而非与 worker（tool-calling 主 agent）直接对话**。

秘书模式至少满足：
- 仍可正常发送消息并接收秘书汇报（best-effort）
- 默认隐藏低频/高级区域（例如历史侧栏、模型/工具选择、trace/工具细节、任务面板等，best-effort）
- 提供可发现的一键入口切换回完整模式
- 支持通过独立路由直接进入秘书模式（例如 `/secretary`，best-effort）
- **会话分离**：秘书模式使用 secretary session；完整模式使用 assistant session；两者 message list 不得互相污染（best-effort）

#### Scenario: Secretary mode routes messages to the secretary backend
- **GIVEN** 用户已进入秘书模式（例如访问 `/secretary`）
- **WHEN** 用户发送一条消息
- **THEN** 前端将消息写入 secretary inbox API（best-effort）
- **AND** 后端在短暂静默窗口后产出一条 batched 的秘书汇报（best-effort）
- **AND** 该过程不应调用 worker chat API（best-effort）

#### Scenario: Full mode routes messages to the worker backend
- **GIVEN** 用户处于完整模式（例如访问 `/chat`）
- **WHEN** 用户发送一条消息
- **THEN** 前端使用 worker chat API 获取流式回复（best-effort）
- **AND** 该过程不应写入 secretary inbox（best-effort）

#### Scenario: Switching modes does not mix sessions
- **GIVEN** 用户在完整模式下已有一个 assistant chat session（best-effort）
- **WHEN** 用户切换到秘书模式
- **THEN** UI 切换到 secretary session 的消息流（best-effort）
- **WHEN** 用户再切换回完整模式
- **THEN** UI 恢复到先前的 assistant session（best-effort）

### Requirement: Secretary mode MUST support multi-message sending with quick acks and batched triage replies (best-effort)
系统必须 (MUST) 在秘书模式下支持“微信式连续发送”：用户可以连续发送多条消息，而不被“assistant 正在生成中”的状态阻塞；系统应该 (SHOULD) 在短暂静默窗口后对“自上次归并以来的消息集合”输出一条低噪声的秘书式汇报（best-effort），而不是逐条进行完整分析回复。

系统可以 (MAY) 提供 quick ack（best-effort），但不得 (MUST NOT) 输出机械的计数式/空洞式回执（例如仅“收到/我继续推进/有 N 个问题”等），且 quick ack 缺失不应阻塞 triage（best-effort）。

#### Scenario: User can send multiple messages without being blocked
- **GIVEN** 用户处于秘书模式（best-effort）
- **WHEN** 用户在短时间内连续发送多条消息
- **THEN** UI 允许每条消息都被发送并显示在对话中（best-effort）
- **AND** 系统不会因为正在生成而阻止用户继续发送（best-effort）

#### Scenario: Secretary produces a single batched reply for a message burst
- **GIVEN** 用户处于秘书模式并连续发送了多条消息（best-effort）
- **WHEN** 用户停止输入并产生短暂静默窗口（best-effort）
- **THEN** 系统输出一条秘书式汇报，覆盖这批消息的归并理解（best-effort）
- **AND** 汇报必须明确下一步（继续推进什么 / 用户需要回复什么）（best-effort）
