## MODIFIED Requirements

### Requirement: Release artifact workflow
The project SHALL build Windows release bundles that can include PortableGit, and the PortableGit resolution SHALL be reproducible and testable.

#### Scenario: Pin PortableGit URL
- **GIVEN** a pinned PortableGit URL is configured (via env or a config file)
- **WHEN** the release script builds a Windows bundle with PortableGit enabled
- **THEN** the script uses the pinned URL and does not depend on “latest” GitHub API resolution

#### Scenario: Self-test resolution logic
- **WHEN** CI runs the release script self-test
- **THEN** it validates the PortableGit URL resolution precedence (env > file > GitHub API)

