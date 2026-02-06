## ADDED Requirements

### Requirement: Worktree mode MUST fail closed for non-git workspaces unless an explicit fallback is configured
When workspace execution mode is `worktree` and the workspace is not a git repository, the system MUST fail with an actionable error by default, and MUST NOT silently fall back to mutable in-place execution.

#### Scenario: Non-git workspace in worktree mode is rejected by default
- **GIVEN** workspace execution mode is configured as `worktree`
- **AND** the target workspace is not a git repository
- **WHEN** a new mutating attempt is created
- **THEN** the system returns an actionable error
- **AND** no mutating attempt is started in-place

### Requirement: Worktree execution root MUST remain the default write boundary for file and command tools
For attempts running in worktree mode, file tools and command tools MUST use `worktree_root` as their default scope and write boundary (subject to policy), not the original workspace root.

#### Scenario: Tool writes are scoped to worktree_root
- **GIVEN** an attempt is running in worktree mode
- **WHEN** a mutating file or command tool is executed
- **THEN** writes are constrained to `worktree_root` by default
- **AND** attempts to escape that root are rejected by policy or boundary checks
