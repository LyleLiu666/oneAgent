## ADDED Requirements
### Requirement: Secretary MUST mediate task recovery as a dispatch-only conversation (best-effort)
当后台 worker task 进入“需要处理”的终态时，系统必须 (MUST) 由秘书在对话区转达可操作信息，并将用户回复转化为一次可追溯的继续推进（resume）动作（best-effort）。

该能力必须 (MUST) 满足（best-effort）：
- **不执行工具**：秘书层不得执行 tool calling；仅允许读取状态/证据指针并 dispatch（例如 `tasks.resume`）。
- **证据优先**：转达内容优先使用 task attempt 的 `summary/observer.reason/observer.next_steps/questions_for_user` 等结构化字段；不需要读取大文件内容（best-effort）。
- **可追溯**：秘书转达消息必须携带对 `task_id/attempt_id` 的可追溯引用；由用户回复触发的 resume 必须在 events/receipt 中记录来源（best-effort）。
- **折叠直通入口**：对用户直通的 artifacts/排障入口应默认折叠；秘书负责默认转达（best-effort）。
- **逐一请示**：当同时存在多个需要用户介入的事项时，秘书必须以“我这里有 N 个事情，接下来一个个请示”的方式逐一推进（best-effort）。

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
- **AND** 系统不在该流程中执行任何 tool calling（best-effort）

#### Scenario: Secretary queues multiple recovery briefs and asks one by one (best-effort)
- **GIVEN** 同一时刻存在多个需要用户介入的 tasks（best-effort）
- **WHEN** 系统需要向用户请示（best-effort）
- **THEN** 秘书生成一个队列并逐一在对话中请示（best-effort）
- **AND** 用户对当前请示的回复不会被错误绑定到其它 task（best-effort）
