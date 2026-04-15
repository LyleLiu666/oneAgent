## ADDED Requirements
### Requirement: External SDK dependencies MUST be pinned by stable snapshots by default
The project MUST resolve `agentsdk` and `memorySdk` from deterministic, committed SDK snapshots by default, rather than from developer-local SDK working directories.

The committed repository state MUST NOT require a separate developer-local checkout of those SDK repositories for the default Go or Docker build path.

#### Scenario: Default backend dependency resolution uses pinned SDK snapshots
- **GIVEN** the committed repository state
- **WHEN** a developer runs the default backend dependency resolution flow
- **THEN** `agentsdk` and `memorySdk` are resolved from the pinned SDK snapshots committed with the repo
- **AND** the flow does not depend on SDK directories outside the repo

#### Scenario: Default Docker build does not require local SDK worktrees
- **GIVEN** the committed repository state
- **WHEN** a developer runs the default Docker compose build path
- **THEN** the backend image resolves SDK dependencies from pinned SDK snapshots
- **AND** the compose file does not require local SDK build contexts by default

### Requirement: Local SDK source overrides MUST be explicit and opt-in
The project MUST provide an explicit local-override path for SDK联调, but that path MUST be opt-in and separate from the default build flow.

#### Scenario: Developer opts into local SDK Docker override
- **GIVEN** a developer wants to联调 local `agentsdk` or `memorySdk`
- **WHEN** the developer enables the documented `sdk-local` Docker override
- **THEN** the backend build resolves SDK dependencies from the specified local SDK directories
- **AND** the default compose file remains unchanged for other developers

#### Scenario: Developer opts into local Go workspace override
- **GIVEN** a developer wants to联调 local `agentsdk` or `memorySdk` during host-side Go development
- **WHEN** the developer creates a local `go.work`
- **THEN** the override remains local-only
- **AND** the committed repository state continues to use pinned SDK snapshots by default
