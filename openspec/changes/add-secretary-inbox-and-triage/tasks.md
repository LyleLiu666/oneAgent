## 1. Spec & Design (planning first)
- [ ] 1.1 Define secretary multi-message + quick ack + batched triage reply requirement in `chat-ux` delta
- [ ] 1.2 Add new spec `system-secretary-orchestration` (inbox, triage, auto-dispatch, idempotency, workspace allocation)
- [ ] 1.3 Write `design.md` (API shapes, storage, idempotency keys, debounce, workspace pool)

## 2. Implementation (after proposal approval)
- [ ] 2.1 Backend: add `internal/secretary` orchestrator module (triage + state)
- [ ] 2.2 Backend: add endpoints `/api/secretary/inbox/messages`, `/api/secretary/triage`, `/api/secretary/state`
- [ ] 2.3 Backend: inbox append returns quick ack (**LLM-generated, no tools**) and persists it as an assistant message for replay
- [ ] 2.4 Backend: triage calls LLM for structured plan, allocates/creates workspaces, auto-creates/enqueues tasks, appends low-noise summary message to session
- [ ] 2.5 Frontend: secretary mode send uses append+ack; debounce triggers triage; allow multiple sends while triage running
- [ ] 2.6 Back-compat: full mode chat stays on `/api/chat`; existing manual handoff remains available (best-effort)

## 3. Tests & Validation
- [ ] 3.1 Backend tests: inbox quick ack + triage idempotency + auto-dispatch + workspace creation
- [ ] 3.2 Frontend tests: multiple sends not blocked + per-message ack + debounce triggers single triage
- [ ] 3.3 `go test ./...` (backend)
- [ ] 3.4 `npm test -- --run` (frontend)
- [ ] 3.5 `openspec validate add-secretary-inbox-and-triage --strict --no-interactive`
