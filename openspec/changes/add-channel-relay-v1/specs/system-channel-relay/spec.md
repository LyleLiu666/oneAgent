## ADDED Requirements

### Requirement: Channel relay MUST ingest inbound messages into secretary inbox with idempotency
The system MUST support channel ingress that maps inbound provider messages into secretary inbox messages, and MUST deduplicate by provider message identity to avoid duplicate dispatch.

#### Scenario: Duplicate inbound webhook does not create duplicate secretary messages
- **GIVEN** a channel provider retries the same inbound message
- **WHEN** relay receives duplicate payloads with the same provider message id
- **THEN** only one secretary inbox append is persisted (best-effort)

### Requirement: Channel relay MUST send outbound task status notifications with traceable references
The system MUST support outbound notifications to the originating channel thread when related tasks reach terminal state, including traceable references (`task_id`, `attempt_id`, artifact pointers best-effort).

#### Scenario: Task completion sends outbound notification to source thread
- **GIVEN** an inbound channel message created or influenced task T
- **WHEN** task T reaches terminal state
- **THEN** relay sends a notification to the mapped channel thread
- **AND** notification includes task and artifact references (best-effort)

### Requirement: Channel relay MUST enforce secure ingress verification
Inbound relay endpoints MUST verify provider authenticity (for example signature/token checks) and reject unverifiable requests.

#### Scenario: Invalid signature is rejected
- **GIVEN** an inbound webhook request with invalid signature
- **WHEN** relay verifies the request
- **THEN** the request is rejected
- **AND** no secretary message or task action is created
