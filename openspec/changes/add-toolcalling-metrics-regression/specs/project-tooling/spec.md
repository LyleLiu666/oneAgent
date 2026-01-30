## ADDED Requirements

### Requirement: Toolcalling reliability MUST be measurable and regression-testable
The project MUST provide a repeatable way to measure and compare “toolcalling smoothness / success rate” across changes, so regressions can be detected without relying on subjective impressions.

At minimum, the project MUST define and report the following metrics (best-effort where noted):
1. **Tool selection accuracy** (best-effort)
2. **Tool arguments validity rate** (JSON validity + required-field missing rate)
3. **Tool execution success rate** (with breakdown: policy denied vs invalid arguments vs tool error)
4. **Steps per task** (average tool loop turns for a fixed task set)

Reference (details/source-of-truth):
- `docs/oneAgent_toolcall_advice/docs/06_测试与指标_把成功率变成可回归的数字.md`

#### Scenario: CI run outputs a machine-readable toolcalling metrics report
- **GIVEN** a fixed scripted regression task set (prompts + deterministic fixtures; best-effort)
- **WHEN** CI runs the toolcalling regression suite
- **THEN** it produces a machine-readable report artifact (JSON/JSONL/SQLite; best-effort)
- **AND** the report includes the minimum metric set listed above

### Requirement: Tool loop MUST record failures with a unified schema
When a tool call fails, the system MUST record a normalized event so failures can be aggregated consistently across tool protocols and providers.

The event MUST include (best-effort):
- tool name
- raw arguments (as received from the model)
- error class (invalid_arguments / policy_denied / tool_error / provider_error / unknown)
- provider + model identifier
- sampling parameters relevant to toolcalling stability (temperature/top_p; best-effort)

#### Scenario: JSON tool loop records invalid arguments failures
- **GIVEN** the JSON native tools loop receives a tool call with invalid arguments payload
- **WHEN** the system rejects the call before tool execution
- **THEN** a normalized failure event is recorded with `error_class=invalid_arguments` (or equivalent)

### Requirement: Tool arguments MUST be schema-validated before execution (required fields at minimum)
Before executing a tool, the system MUST validate tool arguments against the tool’s JSON schema (at least required fields) and reject invalid calls early with a structured, actionable error (best-effort for full schema).

#### Scenario: Missing required fields are rejected without executing the tool
- **GIVEN** a tool definition marks `file_path` as required
- **WHEN** the model issues a tool call without `file_path`
- **THEN** the system rejects the call as invalid arguments
- **AND** the tool handler is not executed
