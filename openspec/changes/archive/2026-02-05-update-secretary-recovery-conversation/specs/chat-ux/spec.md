## MODIFIED Requirements
### Requirement: Secretary mode MUST surface low-noise recovery actions for failed tasks (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下，以低噪声方式提示“需要处理”的失败任务，并提供可恢复动作（best-effort），避免用户在“只聊天”心智下错过失败与下一步。

至少包括（best-effort）：
- 失败任务的提示（基于 Task Queue 中终态且非 `succeeded` 的 attempt）
- **秘书转达**：在对话区转达“发生了什么 + 下一步 + 需要用户确认的问题（若有）”，并附可追溯引用（findings/trace/diff，best-effort）
- 一键继续（`POST /api/tasks/:id/resume`；支持携带 `review_notes`，best-effort）
- 排障入口（优先在秘书模式内打开 trace；无 trace 时再进入完全模式排障，best-effort）
- **直通入口折叠**：findings/diff/trace 等细节入口不得默认显式展示；应放在“更多/展开”中（progressive disclosure，best-effort）
- **具体而非报数**：当同一时间存在多个待处理事项时，系统不得只报数量；必须给出每个事项的具体“原因/下一步/需要你确认什么”（best-effort）

#### Scenario: Failed task is surfaced in secretary mode
- **GIVEN** `GET /api/tasks` 返回至少 1 个任务，其 latest attempt 处于失败终态（非 `queued/running` 且非 `succeeded`）
- **WHEN** 用户处于秘书模式并停留在对话主界面
- **THEN** 页面展示该任务的低噪声“需要处理”提示（best-effort）

#### Scenario: Secretary relays failure reason and next steps in chat (best-effort)
- **GIVEN** 某任务 latest attempt 从 `queued/running` 跃迁到“需要处理终态”（例如 `failed/limit_exceeded/timed_out/interrupted`，best-effort）
- **AND** 用户处于秘书模式且该跃迁发生在本次进入页面之后（不回放历史，best-effort）
- **WHEN** UI 刷新任务列表并检测到该跃迁（best-effort）
- **THEN** 对话区追加一条低噪声 assistant 消息用于“秘书转达”（best-effort）
- **AND** 该消息包含用户可读的原因与下一步（优先使用 `attempt.summary` 与 `attempt.observer.next_steps`，best-effort）
- **AND** 该消息包含可追溯引用入口（例如 findings/trace/diff），但这些入口必须默认折叠（best-effort）

#### Scenario: User can reply to resume a failed task with review_notes (best-effort)
- **GIVEN** 对话区存在一条与 task T 绑定的“秘书转达”消息（best-effort）
- **WHEN** 用户在对话区回复一条消息作为补充信息/决策（best-effort）
- **THEN** 系统调用 `POST /api/tasks/:id/resume` 且 `id=T`（best-effort）
- **AND** 系统将用户回复注入该次 resume 的 `review_notes`（best-effort）
- **AND** 对话区追加一条低噪声回执消息，留痕“已继续推进 + 绑定的 task/attempt”（best-effort）

#### Scenario: Multiple failed tasks are surfaced with a focused current item (best-effort)
- **GIVEN** 用户处于秘书模式（best-effort）
- **AND** 同一时间存在 N 个“需要处理”的任务（N>=2，best-effort）
- **WHEN** UI 检测到这些任务需要用户介入（best-effort）
- **THEN** 对话区至少追加 1 条“秘书转达”消息，且每个事项包含原因/下一步/问题（best-effort）
- **AND** UI 默认聚焦到一个当前事项（例如最新/最相关，best-effort），并允许用户切换要处理的事项（best-effort）

#### Scenario: Troubleshoot opens trace inline when available (best-effort)
- **GIVEN** 失败任务提示已展示（best-effort）
- **AND** 该 attempt 存在 `trace_log_path`（best-effort）
- **WHEN** 用户点击“排障”
- **THEN** UI 在秘书模式内打开 trace 预览（best-effort）
- **AND** 不切换到完全模式（best-effort）

#### Scenario: Troubleshoot enters full mode when no trace is available (best-effort)
- **GIVEN** 失败任务提示已展示（best-effort）
- **AND** 该 attempt 不存在 `trace_log_path`（best-effort）
- **WHEN** 用户点击“排障”
- **THEN** 系统切换到完全模式（best-effort）
- **AND** 跳转到任务工作台页面（例如 `/tasks`，best-effort）
