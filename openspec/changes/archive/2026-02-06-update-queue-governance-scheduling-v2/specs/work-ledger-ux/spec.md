## ADDED Requirements

### Requirement: Queue governance status MUST be visible as a low-noise summary (best-effort)
Workbench and ledger-adjacent views MUST expose a low-noise governance summary for users who run multiple workspaces, including at least active slots, deferred workspace count, and paused workspaces (best-effort).

#### Scenario: User sees governance summary without opening raw events
- **GIVEN** multiple workspaces are competing for queue slots
- **WHEN** user opens task/ledger workbench views
- **THEN** UI shows a compact governance summary (best-effort)
- **AND** user can expand into detailed events if needed
