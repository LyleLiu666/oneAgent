## ADDED Requirements

### Requirement: MCP server MUST provide a policy-governed action plane for task lifecycle operations
In addition to read-only resources, the MCP server MUST support task lifecycle actions (`create`, `resume`, `cancel`) under the same auth and policy boundaries as core runtime APIs.

#### Scenario: Authorized MCP client creates a task
- **GIVEN** an MCP client is authenticated and authorized
- **WHEN** it invokes task create action via MCP
- **THEN** the system creates and enqueues a task
- **AND** action metadata is recorded in evidence/trace (best-effort)

#### Scenario: Unauthorized action request is rejected
- **GIVEN** an MCP client lacks required permission for task resume
- **WHEN** it invokes the resume action
- **THEN** the system rejects the request with actionable unauthorized/policy error

### Requirement: MCP action methods MUST return traceable, user-safe errors
MCP action failures MUST include stable error codes and traceable references, and MUST avoid leaking internal sensitive payloads.

#### Scenario: Approval-required action returns actionable status
- **GIVEN** a policy requires approval for a high-risk action
- **WHEN** an MCP client invokes that action without approval
- **THEN** the server returns an approval-required response with traceable metadata (best-effort)
