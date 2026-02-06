## ADDED Requirements

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
