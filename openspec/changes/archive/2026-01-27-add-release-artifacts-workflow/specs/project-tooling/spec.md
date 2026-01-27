## ADDED Requirements

### Requirement: Release artifact workflow
The project SHALL provide a GitHub Actions workflow that builds cross-platform release artifacts and publishes them as downloadable artifacts for each workflow run.

#### Scenario: Manual release build
- **WHEN** a user triggers the workflow manually
- **THEN** the workflow builds cross-platform binaries and Windows zip bundles
- **AND** it publishes the artifacts and checksums as workflow artifacts

#### Scenario: Tag build
- **WHEN** a tag starting with `v` is pushed
- **THEN** the workflow builds the same release artifacts for that version

### Requirement: Checksums
The workflow SHALL generate and publish a checksums file for the release artifacts.

#### Scenario: Checksums published
- **WHEN** release artifacts are built
- **THEN** the workflow produces `checksums_<version>.txt` and publishes it with the artifacts

