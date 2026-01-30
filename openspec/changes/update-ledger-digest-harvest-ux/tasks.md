## 1. Specs
- [ ] 1.1 Add `system-work-ledger` delta requirements for structured digest + failure clustering + batch follow-up
- [ ] 1.2 Add `work-ledger-ux` delta requirements for harvest mode UX (filter/cluster/batch follow-up)
- [ ] 1.3 Run `openspec validate update-ledger-digest-harvest-ux --strict --no-interactive`

## 2. Backend
- [ ] 2.1 Digest data model: persist `digest.json` alongside markdown (items + clusters + pointers)
- [ ] 2.2 API: fetch structured digest (read-only) without forcing refresh (best-effort)
- [ ] 2.3 API: batch follow-up endpoint that enqueues a task from selected receipt IDs
- [ ] 2.4 Tests: digest structure stability + follow-up enqueue invariants

## 3. Frontend
- [ ] 3.1 Digest harvest view: sections + filters + failure clusters + quick open receipt
- [ ] 3.2 Batch follow-up: multi-select + “Create follow-up task” action + preview payload
- [ ] 3.3 In-app notification: badge/empty-state guidance when digest ready (best-effort)
- [ ] 3.4 Frontend tests: filtering + selection + follow-up request

## 4. Validation
- [ ] 4.1 `cd backend && go test ./...`
- [ ] 4.2 `cd frontend && npm test -- --run`

