## ADDED Requirements

### Requirement: Live LLM regression suite MUST be runnable against configured provider (opt-in)
The project MUST provide a repeatable way to run a “real provider” regression suite locally, using the LLM provider/model configured in the local settings DB, in order to measure stability and performance in real conditions.

This suite MUST be **opt-in** (never runs by default in CI), and MUST produce a machine-readable report plus a human-readable summary.

The suite MUST NOT leak secrets (e.g. API keys) in logs or reports.

#### Scenario: Run live suite and produce reports
- **GIVEN** `.oneagent/settings.db` contains at least one enabled provider + default model
- **WHEN** the developer enables the live suite (env flag) and runs backend tests
- **THEN** the suite performs real `/api/chat` runs against the configured provider
- **AND** it outputs a JSON report and a Markdown summary under `.oneagent/tmp/`
- **AND** the report contains only redacted provider metadata (no API key)

#### Scenario: Tool protocol comparison includes long-text arguments
- **GIVEN** the suite includes a long-text tool-argument task (>=3000 runes)
- **WHEN** the suite runs the task with `tool_protocol=json` and `tool_protocol=xml`
- **THEN** the report records success rate and latency metrics for both protocols

