## ADDED Requirements

### Requirement: Task attempts MUST emit a schema-versioned artifact manifest
Each terminal attempt MUST emit an artifact manifest with an explicit schema version, so downstream APIs and UIs can consume deliverables deterministically.

The manifest MUST expose stable pointers for core evidence fields:
- `summary`
- `findings_path`
- `trace_log_path`
- `changed_files_path` and/or `diff_patch_path`
- `test_report_path` (when available)

#### Scenario: Terminal attempt includes manifest version and stable fields
- **GIVEN** an attempt reaches a terminal state
- **WHEN** artifacts are persisted
- **THEN** the system writes an artifact manifest with explicit `version`
- **AND** core evidence fields are present or marked as unavailable with reasons (best-effort)

### Requirement: Missing artifact pointers MUST carry actionable reason codes
When a recommended artifact pointer is unavailable (for example non-git diff or missing test framework), the system MUST include an actionable reason code and hint instead of silent omission.

#### Scenario: Non-git attempt omits diff patch with explicit reason
- **GIVEN** an attempt runs in a non-git workspace
- **WHEN** artifacts are finalized
- **THEN** `diff_patch_path` may be absent
- **AND** the manifest includes a reason code and hint explaining why diff patch is unavailable
