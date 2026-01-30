## 1. Specs
- [x] 1.1 Add `system-task-queue` delta requirements for queue policy, controls, and schedules
- [x] 1.2 Run `openspec validate add-multi-workspace-queue-governance --strict --no-interactive`

## 2. Backend
- [x] 2.1 Policy model: global limits + per-workspace overrides (defaults preserve current behavior)
- [x] 2.2 Scheduler: enforce policy (fairness/priority) without breaking FIFO per workspace
- [x] 2.3 Queue controls: pause/resume workspace, set priority (best-effort)
- [x] 2.4 Schedules: persist + trigger enqueues (best-effort)
- [x] 2.5 Tests: policy enforcement + pause/resume + schedule trigger

## 3. Frontend
- [x] 3.1 Surface queue policy state (read-only) + explain defaults
- [x] 3.2 Workspace queue controls (pause/resume, priority) (best-effort)
- [x] 3.3 Schedule editor (minimal) (best-effort)
- [x] 3.4 Frontend tests

## 4. Validation
- [x] 4.1 `cd backend && go test ./...`
- [x] 4.2 `cd frontend && npm test -- --run`
