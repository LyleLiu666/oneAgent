## MODIFIED Requirements

### Requirement: 任务成功判定由 Outcome Observer 基于用户预期完成度决定
系统必须 (MUST) 使用一个独立于主/子 Agent 对话上下文的 Outcome Observer 来判定任务是否 `succeeded`：
- Observer 输入至少包含：任务原始描述（用户预期）、workspace 根目录、以及任务产生的产物引用（例如 findings/trace）。
- Observer 必须 (MUST) 输出 `pass/fail` 与可操作原因，并尽可能引用证据（例如相关文件路径/关键变更）。
- 当 `pass=false` 时，Observer 必须 (MUST) 同时输出 `next_steps`（下一步可执行方案），用于指导系统继续交付（见下述自动 follow-up 要求）。
- Observer 应该 (SHOULD) 尽量回答/消化执行过程中的“反问/不确定点”，避免把可由证据推断的决策留给用户。
- 仅当确实需要用户偏好或外部信息时，Observer 才可以 (MAY) 输出 `questions_for_user`（并清晰说明为什么需要）。
- Observer 不得 (MUST NOT) 仅以 “plan/todo 是否完整” 作为成功判定依据（因为 todo 可能不全或不存在）。
- Observer 必须 (MUST) 为只读验收：不得执行命令（例如 `go test`），仅可通过读取文件/列目录/搜索等只读方式获取证据。

#### Scenario: Observer fail includes next_steps
- **GIVEN** attempt 的交付产物不足以满足用户预期
- **WHEN** Outcome Observer 判定 `pass=false`
- **THEN** Observer 输出包含 `reason/evidence`
- **AND** 输出包含可执行的 `next_steps`（可以直接写入下一轮 attempt 的 review_notes）

## ADDED Requirements

### Requirement: Observer fail MUST trigger bounded auto follow-up attempt
当一个 attempt 的执行结束且 Outcome Observer 判定 `pass=false` 时，系统必须 (MUST) 自动创建一个 follow-up attempt 并继续执行，以逼近用户预期交付（best-effort）。

自动 follow-up 必须 (MUST) 满足：
- follow-up attempt 的 `review_notes` 由 Observer 的 `next_steps` 自动填充（可附带简短上下文）
- 系统记录可审计事件（例如 `attempt.queued` data 标注 `source=observer`）
- 自动 follow-up 受 `limits.max_auto_attempts` 约束；达到上限后不得继续自动创建 attempt
- 达到上限后，系统必须 (MUST) 向用户清晰呈现最后一次 Observer 的 `reason + evidence + next_steps`（便于用户手动接管）

#### Scenario: Observer fail auto-enqueues follow-up attempt
- **GIVEN** attempt 执行结束且 Observer 判定 `pass=false`
- **WHEN** 该 task 仍未达到 `limits.max_auto_attempts`
- **THEN** 系统自动创建一个新的 attempt（`status=queued`）
- **AND** 该 attempt 的 `review_notes` 包含 Observer `next_steps`
- **AND** 该 task 被重新入队并继续执行

#### Scenario: Auto follow-up stops after max_auto_attempts
- **GIVEN** 某 task 已自动创建了 `limits.max_auto_attempts` 次 follow-up attempt
- **WHEN** 最新 attempt 再次被 Observer 判定为 `pass=false`
- **THEN** 系统不再自动创建新的 attempt
- **AND** UI/接口返回可解释的失败信息（包含 `next_steps` 供用户接管）
