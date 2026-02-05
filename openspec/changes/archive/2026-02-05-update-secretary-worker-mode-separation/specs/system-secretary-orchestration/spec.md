# system-secretary-orchestration Spec Delta

## RENAMED Requirements
- FROM: `### Requirement: Triage MUST NOT execute tools; it only plans and dispatches (best-effort)`
- TO: `### Requirement: Secretary triage MAY use tools but MUST be read-only to user workspace (best-effort)`

## MODIFIED Requirements

### Requirement: The system MUST provide an append-only secretary inbox API with LLM quick acks (no tools)
系统必须 (MUST) 提供一种“收件箱（inbox）”消息写入机制，用于在秘书模式下追加用户消息；该过程不得触发工具执行（tool calling）（best-effort）。

系统可以 (MAY) 返回 quick ack（best-effort），但 quick ack 不得是机械回执/空洞回执，并且 quick ack 的缺失不得阻塞后续 triage（best-effort）。

#### Scenario: Inbox append persists user message (append-only)
- **GIVEN** 用户在秘书模式下发送一条消息（best-effort）
- **WHEN** 客户端调用 inbox append API（best-effort）
- **THEN** 系统追加一条 `role=user,type=text` 的会话消息（best-effort）
- **AND** 该过程不产生 `tool_call/tool_result` 类型消息（best-effort）

### Requirement: Secretary triage MAY use tools but MUST be read-only to user workspace (best-effort)
系统必须 (MUST) 允许秘书在 triage/汇报阶段使用工具来完成“查询/解释/排障/协调”（best-effort），以提升 agentic 与自愈能力；但系统必须 (MUST) 在权限层面保证：
- 秘书默认 **不具备用户 workspace 的写权限**（不得增删改文件；fail-closed）
- 当需求涉及写文件/改文件/删文件，或预计超出简单 tool loop 预算时，秘书应派工给 worker tasks（或引导切换到完整模式）

#### Scenario: Secretary uses tools to answer a simple progress question
- **GIVEN** 用户在秘书模式下询问“任务进度/任务状态”这类简单问题（best-effort）
- **WHEN** 系统执行 secretary triage（best-effort）
- **THEN** 秘书可以调用任务查询类工具获取证据（best-effort）
- **AND** 最终汇报中包含任务 id/状态/关键产物路径等可追溯信息（best-effort）

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

### Requirement: Secretary MUST ask concrete questions with context, not just counts (best-effort)
当秘书需要用户确认才能继续推进时，系统必须 (MUST) 让秘书以“可执行”的方式提问（best-effort）：
- 不得 (MUST NOT) 只输出“有 N 个问题需要确认/有 N 个任务”等计数式回执作为唯一信息
- 必须 (MUST) 列出具体问题（或可点击的待确认项），并附带相关上下文/证据（例如 worker 的原话、关键日志片段、文件路径、链接）（best-effort）
- 若存在默认选项，秘书应该 (SHOULD) 明确默认值与风险提示，并说明“你不回复我也会按默认继续”（放权与信任，best-effort）

#### Scenario: Confirmation UI shows the actual pending questions
- **GIVEN** 某个 worker task 或秘书自身决策产生待确认项（best-effort）
- **WHEN** 秘书向用户请求确认（best-effort）
- **THEN** 用户能在对话/弹窗中看到每条待确认项的完整内容（best-effort）
