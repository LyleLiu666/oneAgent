## ADDED Requirements

### Requirement: Ledger Daily Status Summary API
The system MUST expose a read-only endpoint `GET /api/ledger/status/today` that summarizes today's Work Ledger status for the current principal.

#### Scenario: Returns status without side effects
- **GIVEN** the user has existing Work Ledger data
- **WHEN** the client requests `GET /api/ledger/status/today`
- **THEN** the response MUST include `day_key`, `digest_exists`, `learning_job_status`, and `sop_proposed_count`
- **AND** the endpoint MUST NOT create or refresh a digest
- **AND** the endpoint MUST NOT start or enqueue a learning job

### Requirement: Ledger Status Badges
The Work Ledger UI MUST surface badges that make today's status visible at a glance.

#### Scenario: Badges indicate new/active items
- **GIVEN** `GET /api/ledger/status/today` reports `digest_exists=true`
- **THEN** the Digest tab MUST show a "ready" badge
- **GIVEN** `sop_proposed_count > 0`
- **THEN** the SOP tab MUST show a count badge
- **GIVEN** `learning_job_status` is `queued` or `running`
- **THEN** the Ledger page MUST show a "learning running" badge or equivalent indicator
