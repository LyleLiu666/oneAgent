## ADDED Requirements

### Requirement: Secretary orchestration MUST preserve source-channel metadata for relay-driven conversations (best-effort)
When secretary messages originate from channel relay ingress, the system MUST preserve source metadata (provider, channel id, thread id, message id best-effort) so follow-up dispatch and completion notifications can be routed back correctly.

#### Scenario: Relay-ingested secretary message keeps source metadata
- **GIVEN** a message is ingested through channel relay into secretary inbox
- **WHEN** secretary triage creates tasks from that message
- **THEN** source-channel metadata remains traceable through triage and task linkage (best-effort)
