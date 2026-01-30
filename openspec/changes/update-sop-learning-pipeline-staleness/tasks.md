## 1. Specs
- [x] 1.1 Add `system-work-ledger` delta requirements for learning pipeline governance hints + evidence invariants
- [x] 1.2 Add `system-skill-management` delta requirements for staleness signals + deprecate/archive workflow
- [x] 1.3 Run `openspec validate update-sop-learning-pipeline-staleness --strict --no-interactive`

## 2. Backend
- [x] 2.1 Record skill usage signals (last_used_at, used_count) (best-effort)
- [x] 2.2 Staleness detection job (best-effort) + API to list stale candidates
- [x] 2.3 Learning job output: attach similar/merge hints and evidence summary (best-effort)
- [x] 2.4 Governance actions: deprecate/archive with reason; ensure recall excludes archived/deprecated
- [x] 2.5 Tests: staleness classification + archive invariants + recall exclusion

## 3. Frontend
- [x] 3.1 Surface stale skills list + recommended actions (archive/keep/merge) (best-effort)
- [x] 3.2 SOP/skills governance UI: show “similar/merge hints” inline (best-effort)
- [x] 3.3 Frontend tests

## 4. Validation
- [x] 4.1 `cd backend && go test ./...`
- [x] 4.2 `cd frontend && npm test -- --run`
