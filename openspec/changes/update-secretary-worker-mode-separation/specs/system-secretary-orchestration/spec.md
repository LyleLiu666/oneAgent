# system-secretary-orchestration Spec Delta

## MODIFIED Requirements

### Requirement: The system MUST provide an append-only secretary inbox API (no tools)
系统必须 (MUST) 提供一种“收件箱（inbox）”消息写入机制，用于在秘书模式下追加用户消息；该过程不得触发工具执行（tool calling）（best-effort）。

系统可以 (MAY) 返回 quick ack（best-effort），但 quick ack 不得是机械回执/空洞回执，并且 quick ack 的缺失不得阻塞后续 triage（best-effort）。

#### Scenario: Inbox append persists user message (append-only)
- **GIVEN** 用户在秘书模式下发送一条消息（best-effort）
- **WHEN** 客户端调用 inbox append API（best-effort）
- **THEN** 系统追加一条 `role=user,type=text` 的会话消息（best-effort）
- **AND** 该过程不产生 `tool_call/tool_result` 类型消息（best-effort）

## ADDED Requirements

### Requirement: Secretary APIs MUST be isolated from worker chat sessions (module boundary)
系统必须 (MUST) 将“秘书会话”与“worker chat 会话”视为不同的 session space：
- Secretary inbox/triage/state 只能读写 `module=secretary` 的会话（best-effort）
- 系统必须 (MUST) 为每个 principal 使用 canonical secretary session id（best-effort stable）
- 客户端传入的任意 `session_id` 不得导致写入到 `module!=secretary` 的会话（best-effort）
- **容错原则（best-effort）**：当客户端误传 `session_id` 时，系统应以 canonical secretary session 为准并继续处理（避免“串台导致留痕分裂”）

#### Scenario: Secretary inbox ignores a non-secretary session id
- **GIVEN** 客户端错误地携带一个 `module=assistant` 的 `session_id` 调用 inbox append（best-effort）
- **WHEN** 系统处理该请求
- **THEN** 系统不应向该 `module=assistant` 会话写入任何消息（best-effort）
- **AND** 系统应忽略该 `session_id` 并改用 canonical secretary session（best-effort）

#### Scenario: Secretary state always refers to canonical secretary session
- **GIVEN** 用户首次进入秘书模式（best-effort）
- **WHEN** 客户端请求 `GET /api/secretary/state`（best-effort）
- **THEN** 系统返回该用户 canonical secretary session id（best-effort）
- **AND** 返回的 triage cursor/triage runs 对应该 canonical secretary session（best-effort）
