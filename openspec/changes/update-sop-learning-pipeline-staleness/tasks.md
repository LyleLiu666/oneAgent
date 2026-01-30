## 1. Specs
- [ ] 1.1 Add `system-work-ledger` delta requirements for learning pipeline governance hints + evidence invariants
- [ ] 1.2 Add `system-skill-management` delta requirements for staleness signals + deprecate/archive workflow
- [ ] 1.3 Run `openspec validate update-sop-learning-pipeline-staleness --strict --no-interactive`

## 2. Backend
- [ ] 2.1 Record skill usage signals (last_used_at, used_count) (best-effort)
- [ ] 2.2 Staleness detection job (best-effort) + API to list stale candidates
- [ ] 2.3 Learning job output: attach similar/merge hints and evidence summary (best-effort)
- [ ] 2.4 Governance actions: deprecate/archive with reason; ensure recall excludes archived/deprecated
- [ ] 2.5 Tests: staleness classification + archive invariants + recall exclusion

## 3. Frontend
- [ ] 3.1 Surface stale skills list + recommended actions (archive/keep/merge) (best-effort)
- [ ] 3.2 SOP/skills governance UI: show “similar/merge hints” inline (best-effort)
- [ ] 3.3 Frontend tests

## 4. Validation
- [ ] 4.1 `cd backend && go test ./...`
- [ ] 4.2 `cd frontend && npm test -- --run`

