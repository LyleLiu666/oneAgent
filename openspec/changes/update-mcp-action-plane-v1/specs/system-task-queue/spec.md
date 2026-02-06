## ADDED Requirements

### Requirement: Task lifecycle actions triggered via MCP MUST be auditable as first-class queue events
When task create/resume/cancel is initiated via MCP, task queue events MUST record source metadata (including MCP origin and principal) so operators can trace who triggered lifecycle transitions.

#### Scenario: MCP resume writes source metadata into task events
- **GIVEN** a terminal task is resumed through MCP action
- **WHEN** the queue creates a new attempt
- **THEN** events include source metadata indicating MCP origin and principal (best-effort)
