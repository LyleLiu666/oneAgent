## ADDED Requirements

### Requirement: Queue governance MUST prevent workspace starvation under sustained load (best-effort)
When global concurrency limits are enabled, the scheduler MUST apply a fairness strategy so lower-priority workspaces are deferred but not starved forever (best-effort).

#### Scenario: Deferred workspace eventually gets a run slot
- **GIVEN** three workspaces have queued tasks and global running slots are limited
- **WHEN** higher-priority workspace keeps receiving new tasks
- **THEN** lower-priority workspaces are deferred
- **AND** deferred workspaces still eventually receive run slots (best-effort)

### Requirement: Scheduled enqueue MUST support misfire policy and idempotent trigger keys (best-effort)
Scheduled task creation MUST define explicit misfire behavior and use an idempotent trigger key to avoid duplicate enqueues for the same schedule window (best-effort).

#### Scenario: Duplicate trigger window does not create duplicate tasks
- **GIVEN** a schedule window is triggered twice due to retry or clock skew
- **WHEN** the scheduler processes both trigger events
- **THEN** only one task is enqueued for that window (best-effort)

### Requirement: Governance decisions MUST be traceable in task events (best-effort)
The scheduler MUST record governance decisions (for example picked, deferred, skipped, paused) with reason codes in task/workspace event streams (best-effort).

#### Scenario: Deferred decision includes reason code
- **GIVEN** a workspace task is not started due to global cap
- **WHEN** scheduler evaluates runnable tasks
- **THEN** an event is recorded with a reason code indicating capacity deferral (best-effort)
