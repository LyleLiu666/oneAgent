# chat-ux Spec Delta

## MODIFIED Requirements

### Requirement: Secretary mode MUST surface low-noise recovery actions for failed tasks (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下，以低噪声方式提示“需要处理”的失败任务，并提供可恢复动作（best-effort），避免用户在“只聊天”心智下错过失败与下一步。

为避免过程性提示污染 secretary chat message list 与模型心智，recovery brief 应遵循（best-effort）：
- recovery brief **仅在面板/卡片中展示**（panel-only）
- 不应 (SHOULD NOT) 将该 recovery brief 作为普通 `assistant text` 消息写入 chat message list（best-effort）
- 需要可追溯引用入口（findings/trace/diff），但默认折叠（best-effort）

#### Scenario: Recovery brief is visible but not persisted into chat message list (best-effort)
- **GIVEN** 某任务 latest attempt 跃迁到“需要处理终态”（例如 `failed/limit_exceeded/timed_out/interrupted`，best-effort）
- **WHEN** 用户处于秘书模式并停留在对话主界面（best-effort）
- **THEN** UI 在面板中展示该任务的低噪声 recovery brief（best-effort）
- **AND** 不向 chat message list 追加新的 `assistant text` 消息（best-effort）

### Requirement: Secretary recovery messages MUST use a concrete action template (best-effort)
Secretary-mode recovery briefs (panel-only) MUST follow a concrete template that includes:
- what failed
- what the system already tried (best-effort)
- what the user can do next
- traceable references (`task_id` / `attempt_id`)

#### Scenario: Recovery brief includes concrete next action (best-effort)
- **GIVEN** a task enters a needs-attention terminal state
- **WHEN** secretary surfaces a recovery brief (panel-only)
- **THEN** the brief includes a concrete next action and traceable references (best-effort)

