## ADDED Requirements

### Requirement: Worktree attempts MUST persist deterministic execution metadata for audit and rollback
When worktree mode is enabled, each mutating attempt MUST persist deterministic execution metadata, including at least `worktree_root`, `base_commit_sha`, and lifecycle outcome of worktree cleanup.

#### Scenario: Attempt artifacts include worktree metadata
- **GIVEN** a workspace uses worktree mode
- **WHEN** the system starts an attempt
- **THEN** attempt artifacts include `worktree_root` and `base_commit_sha`
- **AND** terminal artifacts/receipt record whether cleanup succeeded or failed (best-effort)

### Requirement: Worktree cleanup failures MUST be recoverable and auditable
The system MUST treat worktree cleanup as a managed lifecycle step:
- cleanup failures MUST be recorded with actionable reason
- repeated cleanup attempts SHOULD be supported (best-effort)
- orphan worktrees SHOULD be reclaimed by a sweeper (best-effort)

#### Scenario: Cleanup failure is recorded with actionable hint
- **GIVEN** an attempt has reached terminal state in worktree mode
- **AND** worktree cleanup fails due to file lock or permission issue
- **WHEN** the system finalizes attempt artifacts
- **THEN** it records a cleanup-failed event with actionable remediation hint
- **AND** the attempt remains queryable and resumable
