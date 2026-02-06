# project-tooling Specification

## Purpose
Defines project-level tooling requirements, including CI coverage, release artifact workflows, and regression suites for toolcalling reliability and live-provider runs.
## Requirements
### Requirement: Continuous Integration coverage
The project SHALL run backend unit tests, frontend unit tests, and an end-to-end smoke test in CI for pull requests and pushes.

#### Scenario: PR validation
- **WHEN** a pull request is opened or updated
- **THEN** CI runs backend tests
- **AND** CI runs frontend tests in non-watch mode
- **AND** CI runs an end-to-end smoke test that starts the server and hits core APIs

### Requirement: Daily scheduled regression run
The project SHALL run the same CI test suite on a daily schedule to catch regressions without requiring code changes.

#### Scenario: Scheduled run
- **WHEN** the scheduled workflow triggers
- **THEN** CI runs the backend tests, frontend tests, and end-to-end smoke test

### Requirement: Manual CI trigger
The project SHALL support manually triggering the CI workflow to validate the suite on-demand.

#### Scenario: workflow_dispatch
- **WHEN** a user triggers the workflow manually
- **THEN** CI runs the backend tests, frontend tests, and end-to-end smoke test

### Requirement: Release artifact workflow
The project SHALL build Windows release bundles that can include PortableGit, and the PortableGit resolution SHALL be reproducible and testable.

#### Scenario: Pin PortableGit URL
- **GIVEN** a pinned PortableGit URL is configured (via env or a config file)
- **WHEN** the release script builds a Windows bundle with PortableGit enabled
- **THEN** the script uses the pinned URL and does not depend on “latest” GitHub API resolution

#### Scenario: Self-test resolution logic
- **WHEN** CI runs the release script self-test
- **THEN** it validates the PortableGit URL resolution precedence (env > file > GitHub API)

### Requirement: Checksums
The workflow SHALL generate and publish a checksums file for the release artifacts.

#### Scenario: Checksums published
- **WHEN** release artifacts are built
- **THEN** the workflow produces `checksums_<version>.txt` and publishes it with the artifacts

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

### Requirement: The project MUST provide a repeatable head-to-head benchmark suite for end-to-end delivery quality
The project MUST provide a repeatable benchmark suite that runs a fixed task set and reports objective delivery-quality metrics for comparison across versions.

At minimum, the suite MUST report:
- first pass success rate
- recovery success rate
- human intervention count
- evidence completeness
- cost per successful delivery (best-effort)

#### Scenario: Benchmark run outputs a machine-readable report
- **GIVEN** a defined benchmark task set
- **WHEN** the benchmark suite runs
- **THEN** it outputs a machine-readable report containing all required metrics
- **AND** the report can be compared to a previous baseline run (best-effort)

### Requirement: Benchmark suite MUST support nightly and manual execution without blocking regular PR validation
The project MUST support scheduled and manual benchmark execution paths, and SHOULD avoid making benchmark runtime a mandatory blocker for regular PR CI.

#### Scenario: Nightly benchmark run produces trend artifacts
- **WHEN** the nightly benchmark workflow triggers
- **THEN** the suite runs against the configured task set
- **AND** publishes artifacts that include current values and baseline deltas (best-effort)

#### Scenario: Manual benchmark run is available for release decisions
- **WHEN** a maintainer triggers benchmark execution manually
- **THEN** the same benchmark suite runs with the same metric schema

### Requirement: OpenSpec governance checks MUST prevent placeholder purpose and status drift
The project MUST provide an automated OpenSpec governance check that fails when capability specs keep placeholder `Purpose` values (for example `TBD`) or when roadmap status snapshots drift from command-derived status sources.

#### Scenario: Placeholder Purpose is rejected
- **GIVEN** a capability spec still contains `Purpose: TBD`
- **WHEN** the OpenSpec governance check runs
- **THEN** the check fails with an actionable error pointing to the spec path

#### Scenario: Roadmap status drift is detected
- **GIVEN** roadmap snapshot claims a change is active/completed
- **AND** `openspec list` output does not match that snapshot
- **WHEN** the OpenSpec governance check runs
- **THEN** the check fails with an actionable diff summary (best-effort)

### Requirement: OpenSpec governance checks MUST be runnable in local and CI workflows
The project MUST expose the same OpenSpec governance checks for local developer runs and CI, so “spec truth” is enforced consistently before merge.

#### Scenario: Local and CI run the same governance check entrypoint
- **WHEN** a developer runs the documented local check command
- **THEN** it executes the same validation logic used by CI (best-effort)
- **AND** both paths return non-zero exit code on governance violations

