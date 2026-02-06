## ADDED Requirements

### Requirement: MCP action-plane operations MUST be covered by principal policy and approval gates
The system MUST treat MCP task lifecycle actions as policy-governed mutating operations, including approval constraints when configured.

#### Scenario: MCP resume action is blocked by approval policy
- **GIVEN** policy marks task resume action as approval-required
- **WHEN** MCP client invokes resume without approval grant
- **THEN** the system blocks execution and returns approval-required response
- **AND** the decision is recorded in audit evidence (best-effort)

### Requirement: MCP approvals MUST be scoped and non-replayable
Approval grants for MCP actions MUST be scoped to action fingerprint and MUST NOT be replayable across attempts or altered parameters.

#### Scenario: MCP approval cannot be replayed for a different action payload
- **GIVEN** one MCP action approval grant has been consumed
- **WHEN** client invokes the same action type with a different payload fingerprint
- **THEN** the previous approval is not reused
- **AND** a new approval flow is required (or action is denied by policy)
