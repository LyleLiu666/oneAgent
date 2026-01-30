## 1. Specs
- [ ] 1.1 Add `system-task-queue` delta requirements for queue policy, controls, and schedules
- [ ] 1.2 Run `openspec validate add-multi-workspace-queue-governance --strict --no-interactive`

## 2. Backend
- [ ] 2.1 Policy model: global limits + per-workspace overrides (defaults preserve current behavior)
- [ ] 2.2 Scheduler: enforce policy (fairness/priority) without breaking FIFO per workspace
- [ ] 2.3 Queue controls: pause/resume workspace, set priority (best-effort)
- [ ] 2.4 Schedules: persist + trigger enqueues (best-effort)
- [ ] 2.5 Tests: policy enforcement + pause/resume + schedule trigger

## 3. Frontend
- [ ] 3.1 Surface queue policy state (read-only) + explain defaults
- [ ] 3.2 Workspace queue controls (pause/resume, priority) (best-effort)
- [ ] 3.3 Schedule editor (minimal) (best-effort)
- [ ] 3.4 Frontend tests

## 4. Validation
- [ ] 4.1 `cd backend && go test ./...`
- [ ] 4.2 `cd frontend && npm test -- --run`

