# project-tooling Specification

## Purpose
TBD - created by archiving change add-ci-daily-runs. Update Purpose after archive.
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

