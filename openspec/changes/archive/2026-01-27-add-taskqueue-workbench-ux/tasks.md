## 1. Specs
- [x] 1.1 Add OpenSpec deltas for Workbench UI and OCC preconditions

## 2. Backend
- [x] 2.1 Audit task queue APIs and data model for workspace aggregation needs
- [x] 2.2 If needed: add endpoints/filters for `workspace` + status + pagination
- [x] 2.3 Add/adjust tests for multi-workspace list semantics

## 3. Frontend
- [x] 3.1 Add Workbench view/panel: workspace list + task list + queue indicators
- [x] 3.2 Add actions: create/cancel/resume; show attempt history + events
- [x] 3.3 Add Vitest coverage for key flows

## 4. Validation
- [x] 4.1 `cd backend && go test ./...`
- [x] 4.2 `cd frontend && npm test -- --run`
- [x] 4.3 `scripts/e2e_smoke_test.sh`
- [x] 4.4 `openspec validate add-taskqueue-workbench-ux --strict --no-interactive`
