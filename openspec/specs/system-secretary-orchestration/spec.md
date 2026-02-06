# system-secretary-orchestration Specification

## Purpose
Defines secretary orchestration APIs and behavior (inbox/triage/state), SU/SW separation, shared memory sync, worker dispatch, and traceable recovery mediation.

## Requirements
### Requirement: The system MUST provide an append-only secretary inbox API with LLM quick acks (no tools)
系统必须 (MUST) 提供一种“收件箱（inbox）”消息写入机制，用于在秘书模式下追加用户消息；该过程不得触发工具执行（tool calling）（best-effort）。

系统可以 (MAY) 返回 quick ack（best-effort），但 quick ack 不得是机械回执/空洞回执，并且 quick ack 的缺失不得阻塞后续 triage（best-effort）。

#### Scenario: Inbox append persists user message (append-only)
- **GIVEN** 用户在秘书模式下发送一条消息（best-effort）
- **WHEN** 客户端调用 inbox append API（best-effort）
- **THEN** 系统追加一条 `role=user,type=text` 的会话消息（best-effort）
- **AND** 该过程不产生 `tool_call/tool_result` 类型消息（best-effort）

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

### Requirement: Secretary MUST use a canonical permanent session per principal (best-effort)
系统必须 (MUST) 为每个 `principal_id` 提供一个**唯一且持久化**的 secretary session（best-effort），用于承载秘书与用户/系统之间的长期对话；该 secretary session 在刷新页面、服务重启后仍保持一致（best-effort）。

系统应该 (SHOULD) 允许客户端在调用 secretary 相关 API 时省略 `session_id`，由服务端根据 `principal_id` 解析并返回 canonical `session_id`（best-effort）。

#### Scenario: Secretary session id is stable across reload and restart (best-effort)
- **GIVEN** `principal_id=P` 已产生过一次 secretary 对话（best-effort）
- **WHEN** 用户刷新页面并再次进入秘书模式（best-effort）
- **THEN** 系统解析到同一个 canonical `session_id`（best-effort）
- **WHEN** oneAgent 重启后用户再次进入秘书模式（best-effort）
- **THEN** 系统仍解析到同一个 canonical `session_id`（best-effort）

### Requirement: Secretary orchestration MUST be split into SU/SW channels with distinct responsibilities (best-effort)
系统必须 (MUST) 将秘书编排划分为两个逻辑通道（双分身；best-effort）：
- `Secretary(User)`（SU）：面向用户对话与汇报；默认只读（best-effort）
- `Secretary(Work)`（SW）：面向执行层派工/恢复/答疑；默认只 dispatch，不直接面向用户（best-effort）

系统必须 (MUST) 将用户输入路由给 SU，将 worker 的事件/提问路由给 SW（best-effort）。

#### Scenario: User messages are routed to SU while worker events are routed to SW (best-effort)
- **GIVEN** 同一 `principal_id` 下存在 SU 与 SW 两个通道（best-effort）
- **WHEN** 用户在秘书模式发送一条消息（best-effort）
- **THEN** 系统将该输入交由 SU 处理并生成用户侧回复（best-effort）
- **WHEN** 某 worker 产生“需要秘书介入”的事件或提问（best-effort）
- **THEN** 系统将该输入交由 SW 处理并生成对 worker 的答复或派工（best-effort）

### Requirement: SU/SW MUST share an append-only Memory and perform trigger-based pull-sync (last 10) (best-effort)
系统必须 (MUST) 提供一个 SU/SW 共享的 append-only Memory（best-effort），用于记录：
- 流水账（worklog）
- findings（结论/约束/交付物指针/待办）
- 上下文压缩摘要（context_summary；best-effort）

系统必须 (MUST) 在 SU/SW “准备生成回复”前触发 pull-sync（best-effort）：
- 仅同步对方通道中“尚未同步”的增量（通过 cursor 去重，best-effort）
- **只注入最后 10 条**（按时间/序号排序后取末尾 10 条，best-effort）
- 若存在省略，必须提示省略条数（best-effort）

#### Scenario: SU pull-syncs from SW before replying and reports omitted count (best-effort)
- **GIVEN** SW 写入了 23 条新的 Memory entries，且 SU 尚未同步（best-effort）
- **WHEN** SU 准备对用户生成回复（best-effort）
- **THEN** 系统为 SU 注入来自 SW 的最后 10 条 entries（best-effort）
- **AND** 系统告知“已省略 13 条较早同步”（best-effort）
- **AND** 系统更新 SU→SW 的同步 cursor（best-effort）

### Requirement: Secretary MUST mediate task recovery as an agentic, traceable conversation (best-effort)
当后台 worker task 进入“需要处理”的终态时，系统必须 (MUST) 由秘书在对话区转达可操作信息，并将用户回复转化为一次可追溯的继续推进（resume）动作（best-effort）。同时，秘书必须优先做到“具体清楚 + 可追溯 + 可自愈”，而不是把工程错误或含糊的“数量汇报”抛给用户。

该能力必须 (MUST) 满足（best-effort）：
- **可用工具但禁止文件变更**：秘书允许使用只读/排障类工具获取证据与定位信息（best-effort），但必须禁止对用户目录做增删改（由 policy fail-closed 强制执行，best-effort）。
- **证据优先**：转达内容优先使用 task attempt 的 `summary/observer.reason/observer.next_steps/questions_for_user` 等结构化字段；不需要读取大文件内容（best-effort）。
- **可追溯**：秘书转达消息必须携带对 `task_id/attempt_id` 的可追溯引用；由用户回复触发的 resume 必须在 events/receipt 中记录来源（best-effort）。
- **折叠直通入口**：对用户直通的 artifacts/排障入口应默认折叠；秘书负责默认转达（best-effort）。
- **具体而非报数**：当同时存在多个需要用户介入的事项时，秘书不得只报数量；必须给出每个事项的具体“原因/下一步/需要你确认什么（若有）”，并聚焦到一个当前事项（best-effort）。
- **工程错误先自愈**：当失败原因是协议解析/工具参数等工程性错误时，系统应优先在 agent loop 内自愈重试；仅当超过预算仍无法恢复时，才升级为“需要处理”并向用户呈现（best-effort）。

#### Scenario: Secretary posts a recovery brief for a needs-attention attempt (best-effort)
- **GIVEN** task T 的 latest attempt 进入 `failed/limit_exceeded/timed_out/interrupted` 等“需要处理终态”（best-effort）
- **WHEN** 用户处于秘书模式并可见该任务（best-effort）
- **THEN** 系统生成一条低噪声 recovery brief（秘书转达），包含原因/下一步/问题（best-effort）
- **AND** 该 brief 包含 findings/trace/diff 等证据入口（best-effort）

#### Scenario: User reply triggers a resume dispatch with review_notes (best-effort)
- **GIVEN** 用户对 task T 的 recovery brief 给出回复（best-effort）
- **WHEN** 系统将该回复用于恢复推进（best-effort）
- **THEN** 系统调用 `POST /api/tasks/:id/resume` 且 `id=T`（best-effort）
- **AND** 该次 resume 记录 `review_notes` 与来源为 `secretary-recovery`（best-effort）
- **AND** 如需补充定位信息，秘书可在该流程中调用只读/排障类工具，但不得执行文件变更（best-effort）

#### Scenario: Secretary keeps a focused recovery item and binds user replies correctly (best-effort)
- **GIVEN** 同一时刻存在多个需要用户介入的 tasks（best-effort）
- **WHEN** 系统需要向用户请示（best-effort）
- **THEN** 秘书维护一个“当前聚焦事项”（best-effort）
- **AND** 用户对当前请示的回复不会被错误绑定到其它 task（best-effort）

#### Scenario: Engineer-error failures trigger self-heal retries before surfacing (best-effort)
- **GIVEN** 某任务 attempt 失败原因是“observer 输出不可解析/工具参数不合法”等工程性错误（best-effort）
- **WHEN** 系统进入 recovery 路径（best-effort）
- **THEN** 系统优先把错误作为 `tool_result`/error context 回灌给 agent loop 并尝试自愈重试（best-effort）
- **AND** 仅当重试超过预算仍失败时，才把该 attempt 作为需要用户介入的事项展示（best-effort）

### Requirement: Secretary triage MUST be built on the shared Agent Factory (best-effort)
系统必须 (MUST) 允许 secretary triage（SW planner / SU report）复用共享的 Agent Factory（best-effort），以继承一致的：
- prompt assembly（stable prefix / volatile turn context）
- tool protocol 选择策略（即使 triage 本身不执行工具）
- 留痕（message list + trace pointers）

#### Scenario: Secretary uses the same agent runtime building blocks as worker chat (best-effort)
- **GIVEN** 项目已提供 Agent Factory（best-effort）
- **WHEN** 系统构建 secretary triage planner（SW）与 user-facing secretary（SU）（best-effort）
- **THEN** 其 prompt assembly 与 KV-cache 策略来自同一套基础设施（best-effort）
- **AND** 不会重复实现一套“强制 JSON + 手写 parse”的专用路径（best-effort）

### Requirement: Progress/status questions MUST NOT require brittle keyword routing or extra user clarification (best-effort)
当用户在秘书模式下询问进度/状态/完成情况（例如“任务完成得怎么样”“现在有几个任务在进行”）时，系统必须 (MUST) 直接给出基于系统事实的进度回复（best-effort），而不是反问用户“是哪一个任务/在哪个目录”等无谓澄清，尤其在系统只存在 1 个相关任务时（best-effort）。

系统应该 (SHOULD) 在 triage planner 的上下文中提供 best-effort 的 `tasks_snapshot`（只读快照，易变内容），以减少模型“看不到任务状态而只能反问”的情况。

#### Scenario: Only one task exists; progress question does not trigger clarification (best-effort)
- **GIVEN** 当前 principal 只有 1 个相关任务处于 running/queued/just-finished（best-effort）
- **WHEN** 用户询问“任务完成得怎么样”（best-effort）
- **THEN** 系统返回基于任务状态的进度摘要（best-effort）
- **AND** 不会要求用户先回答“你指的是哪个任务/哪个目录”（best-effort）

### Requirement: Secretary triage MUST express intent via a structured output channel (best-effort)
系统必须 (MUST) 为 secretary triage planner 提供一种结构化方式表达“本轮意图”（best-effort），例如：
- `intent=progress`（只需汇报进度，不派工）
- `intent=dispatch`（生成派工计划并可能产生 questions）
- `intent=clarify`（需要用户确认后再派工）

该 intent 必须通过 tool-call 或宽松 tags 返回（best-effort），以避免在代码层面做 brittle 的 NLP 关键词路由。

#### Scenario: Planner returns progress intent for a progress query (best-effort)
- **GIVEN** 用户询问进度（best-effort）
- **WHEN** triage planner 输出结构化结果（best-effort）
- **THEN** intent=progress（best-effort）
- **AND** SU 根据系统数据生成确定性的进度汇报（best-effort）

### Requirement: Triage MUST surface actionable user confirmations when `questions[]` is non-empty (best-effort)
当 triage 结果包含 `questions[]`（非空）时，系统必须 (MUST) 以“可操作”的方式把这些待确认点呈现给用户（best-effort），避免只返回计数或模糊话术导致用户无法继续。

至少满足（best-effort）：
- **逐条可见**：每个 question 的内容必须对用户可见（在对话摘要或可发现的 UI 入口中至少一种）。
- **下一步明确**：必须说明用户如何回复/如何选择才能继续推进（best-effort）。
- **可追溯恢复**：刷新/重进会话后，用户仍可看到当前未解决的 `questions[]`（best-effort；可通过 `GET /api/secretary/state` 等恢复）。
- **避免内部术语**：秘书对用户的描述应避免“派工/worker/workspace”等内部实现词；如必须涉及路径/目录等概念，应使用用户语言解释（best-effort）。

#### Scenario: Triage returns questions and the user can see exactly what to confirm
- **GIVEN** triage 对某次消息归并输出 `questions[]` 且非空（best-effort）
- **WHEN** 系统写入秘书汇报消息或返回 triage 响应（best-effort）
- **THEN** 用户在秘书模式下可以看到每条问题的具体内容（best-effort）
- **AND** 用户可以从汇报中得知下一步如何回复以继续（best-effort）

#### Scenario: Questions remain visible after refresh (best-effort)
- **GIVEN** triage 产生了未解决的 `questions[]`（best-effort）
- **WHEN** 用户刷新页面或重新进入会话（best-effort）
- **THEN** 系统可通过 secretary state 恢复这些 `questions[]` 以便继续确认（best-effort）

### Requirement: Secretary triage MAY use tools but MUST be read-only to user workspace (best-effort)
系统必须 (MUST) 允许秘书在 triage/汇报阶段使用工具来完成“查询/解释/排障/协调”（best-effort），以提升 agentic 与自愈能力；但系统必须 (MUST) 在权限层面保证：
- 秘书默认 **不具备用户 workspace 的写权限**（不得增删改文件；fail-closed）
- 当需求涉及写文件/改文件/删文件，或预计超出简单 tool loop 预算时，秘书应派工给 worker tasks（或引导切换到完整模式）

#### Scenario: Secretary uses tools to answer a simple progress question
- **GIVEN** 用户在秘书模式下询问“任务进度/任务状态”这类简单问题（best-effort）
- **WHEN** 系统执行 secretary triage（best-effort）
- **THEN** 秘书可以调用任务查询类工具获取证据（best-effort）
- **AND** 最终汇报中包含任务 id/状态/关键产物路径等可追溯信息（best-effort）

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
