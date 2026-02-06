## ADDED Requirements

### Requirement: Secretary recovery messages MUST use a concrete action template (best-effort)
Secretary-mode recovery messages MUST follow a concrete template that includes:
- what failed
- what the system already tried (best-effort)
- what the user can do next
- traceable references (`task_id` / `attempt_id`)

#### Scenario: Recovery message includes concrete next action
- **GIVEN** a task enters a needs-attention terminal state
- **WHEN** secretary posts a recovery brief
- **THEN** the message includes a concrete next action and traceable references (best-effort)

### Requirement: Secretary mode MUST avoid count-only pending prompts when actionable details exist (best-effort)
When there are pending confirmations or recovery items, secretary mode MUST avoid count-only notifications as the only surfaced content if actionable details are available.

#### Scenario: Multiple pending items include actionable summaries
- **GIVEN** multiple pending recovery/confirmation items exist
- **WHEN** secretary mode surfaces them in chat
- **THEN** each surfaced item includes an actionable summary (best-effort)
- **AND** the UI does not only show a raw count
