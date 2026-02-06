## ADDED Requirements

### Requirement: Progress intent responses MUST prefer system facts over clarification when ambiguity is low
For progress/status questions, secretary triage MUST prioritize deterministic answers derived from task state snapshots when ambiguity is low (for example a single relevant task), and MUST avoid unnecessary clarification prompts.

#### Scenario: Single relevant task gets direct progress answer
- **GIVEN** one relevant task exists for the principal
- **WHEN** the user asks a progress/status question
- **THEN** secretary returns a direct status summary based on system facts
- **AND** does not ask "which task" clarification

### Requirement: Engineering failures MUST go through bounded self-heal before user escalation
When failures are caused by engineering/protocol/tool-argument issues, the system MUST attempt bounded self-heal (repair + retry) before surfacing user-facing intervention requests.

#### Scenario: Protocol parse failure triggers self-heal before escalation
- **GIVEN** triage or observer output fails structured parsing
- **WHEN** the recovery pipeline runs
- **THEN** the system performs bounded repair/retry attempts
- **AND** only escalates to user after retry budget is exhausted

#### Scenario: Escalation includes actionable context
- **GIVEN** self-heal budget is exhausted
- **WHEN** secretary escalates to the user
- **THEN** the message includes concrete reason, next step, and task/attempt references (best-effort)
