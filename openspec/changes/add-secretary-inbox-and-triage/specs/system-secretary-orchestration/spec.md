## ADDED Requirements

### Requirement: The system MUST provide an append-only secretary inbox API with LLM quick acks (no tools)
系统必须 (MUST) 提供一种“收件箱（inbox）”消息写入机制，用于在秘书模式下追加用户消息；系统必须 (MUST) 为每条写入生成一个 quick ack（**由 LLM 生成的短句**，best-effort），且该 quick ack 不得触发工具执行（tool calling）。系统应该 (SHOULD) 让 quick ack 与当前消息相关（例如点出/复述 1 个关键信息），而不是固定模板回复（best-effort）。

#### Scenario: Inbox append persists user message and an LLM quick ack
- **GIVEN** 用户在秘书模式下发送一条消息（best-effort）
- **WHEN** 客户端调用 inbox append API（best-effort）
- **THEN** 系统追加一条 `role=user,type=text` 的会话消息（best-effort）
- **AND** 系统调用 LLM 生成 quick ack 文本（best-effort）
- **AND** 系统追加一条 `role=assistant,type=text` 的 ack 会话消息（best-effort）
- **AND** 该过程不产生 `tool_call/tool_result` 类型消息（best-effort）

#### Scenario: Inbox append returns a quick ack
- **GIVEN** 用户在秘书模式下发送一条消息（best-effort）
- **WHEN** 客户端调用 inbox append API（best-effort）
- **THEN** 系统返回一个 quick ack（LLM 生成的短句，best-effort）
- **AND** 该 quick ack 不依赖 triage 完成（best-effort）

#### Scenario: Inbox quick ack is traceable (replayable)
- **GIVEN** 用户在秘书模式下发送消息并收到 quick ack（best-effort）
- **WHEN** 用户刷新页面或重新进入会话（best-effort）
- **THEN** 对话中仍可看到同样的 quick ack 内容（best-effort）

### Requirement: The system MUST provide a triage API that batches messages into a single secretary report and dispatches workers (best-effort)
系统必须 (MUST) 提供一个 triage（归并/派工）机制（best-effort），用于对“自上次 triage 以来的消息集合”生成一条低噪声的秘书汇报，并**默认自动**将可执行工作派发为后台 worker（Task Queue tasks）。

#### Scenario: Triage creates a single batched report and dispatches tasks
- **GIVEN** 某会话中存在多条尚未被 triage 的用户消息（best-effort）
- **WHEN** 客户端调用 triage API（best-effort）
- **THEN** 系统写入一条 `role=assistant,type=text` 的秘书汇报消息（best-effort）
- **AND** 系统默认自动创建并 enqueue 一个或多个后台 tasks 作为 worker（best-effort）
- **AND** 系统更新 triage cursor，使后续 triage 仅处理新增消息（best-effort）

### Requirement: The system MUST allocate workspaces for dispatched workers, and MAY create new workspaces for unscoped work (best-effort)
系统必须 (MUST) 为 triage 派发的 worker tasks 分配 workspace（best-effort），以满足文件工具/命令工具的默认作用域边界。

当用户未指定 workspace 且该工作不依赖既有 repo 时，系统可以 (MAY) 自动创建一个新的目录作为 workspace（best-effort），用于：
- 与代码仓库隔离（降低误改风险）
- 与其它 workstreams 获得并行度（Task Queue 按 workspace 并行）
当系统自动创建 workspace 时，系统必须 (MUST) 支持用 `ONEAGENT_WORKSPACE_POOL_DIR` 指定 workspace pool root；若未设置则默认使用 `ONEAGENT_HOME/.oneagent/workspaces`（best-effort 创建目录）。

#### Scenario: Unscoped non-repo work creates a new workspace
- **GIVEN** 用户在秘书模式下提出一个不依赖既有 repo 的需求（例如整理一份报告）（best-effort）
- **AND** 当前会话未绑定 workspace（best-effort）
- **WHEN** 系统执行 triage 并决定派工（best-effort）
- **THEN** 系统创建一个新的目录作为 workspace 并用于创建 task（best-effort）
- **AND** 系统在秘书汇报或 triage 响应中告知该 workspace 路径（best-effort）
- **AND** 该 workspace 位于 workspace pool root 之下（best-effort）

#### Scenario: Repo-dependent work uses the session workspace or asks the user
- **GIVEN** 用户提出一个看起来依赖既有代码库的需求（例如“改代码/跑测试”）（best-effort）
- **WHEN** 系统执行 triage（best-effort）
- **THEN** 若会话已绑定 workspace，则派发的 tasks 使用该 workspace（best-effort）
- **AND** 若会话未绑定 workspace 且系统无法推断目标 repo，则在 `questions[]` 中向用户询问 workspace 选择（best-effort）

### Requirement: Triage MUST be idempotent for the same input range (no duplicate dispatch)
系统必须 (MUST) 保证 triage 在相同输入范围（例如相同 cursor 且无新增消息）下的幂等性：不得重复创建/重复 enqueue 相同 worker tasks（best-effort）。

#### Scenario: Repeating triage does not create duplicate tasks
- **GIVEN** 某 session 已对同一批消息成功执行过一次 triage（best-effort）
- **WHEN** 客户端用相同 cursor 再次调用 triage API（best-effort）
- **THEN** 系统返回与之前一致的 triage 结果（best-effort）
- **AND** 系统不会重复创建/重复 enqueue 相同的 tasks（best-effort）

### Requirement: Triage MUST record evidence linking messages to dispatched workers (best-effort)
系统必须 (MUST) 记录 triage 的证据链（best-effort），至少包含：
- 本次 triage 覆盖的消息范围（message ids / hash）
- 本次 triage 创建的 task ids（若有）
- 最新 triage cursor

系统必须 (MUST) 提供 best-effort 的查询接口以恢复该状态（例如 `GET /api/secretary/state`）。

#### Scenario: Client can restore triage cursor and created tasks
- **GIVEN** 某 session 已执行过 triage（best-effort）
- **WHEN** 客户端请求 secretary state（best-effort）
- **THEN** 系统返回最新 cursor 与最近一次（或最近 N 次）triage 的 created_task_ids（best-effort）

### Requirement: Triage MUST NOT execute tools; it only plans and dispatches (best-effort)
系统必须 (MUST) 保证 triage 阶段不执行工具调用（tool calling）（best-effort），避免在“秘书编排层”产生不可控副作用；实际执行应由 worker tasks 承担。

#### Scenario: Triage does not emit tool_call/tool_result messages
- **GIVEN** 用户在秘书模式下追加了多条消息（best-effort）
- **WHEN** 系统执行 triage（best-effort）
- **THEN** 会话中不会出现由 triage 产生的 `tool_call/tool_result` 类型消息（best-effort）
