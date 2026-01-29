## 1. Specs
- [ ] 1.1 Add `system-task-queue` delta requirements for waterfall events/log view and tail artifact reading
- [ ] 1.2 Run `openspec validate update-task-events-waterfall --strict --no-interactive`

## 2. Backend
- [ ] 2.1 Add `tail` support to `GET /api/tasks/:id/attempts/:attempt_id/artifacts/:kind` (bounded, best-effort)
- [ ] 2.2 Add Go tests for tail read semantics (small file + large file + missing file)

## 3. Frontend
- [ ] 3.1 Add a reusable event/log viewer component (Pretty/Raw, filter/search, copy, expandable details)
- [ ] 3.2 Wire TaskWorkbench "事件" tab to the new viewer (no flicker on background refresh)
- [ ] 3.3 Wire TaskQueuePanel "事件" section to the new viewer (compact mode)
- [ ] 3.4 Add/adjust Vitest coverage for: filters/search, data expansion, and stale-while-revalidate refresh behavior

## 4. Validation
- [ ] 4.1 `cd backend && go test ./...`
- [ ] 4.2 `cd frontend && npm test -- --run`

