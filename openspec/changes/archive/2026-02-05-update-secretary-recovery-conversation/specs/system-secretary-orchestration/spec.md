## ADDED Requirements
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
