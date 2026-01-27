## ADDED Requirements

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

