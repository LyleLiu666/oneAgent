## 1. Specification
- [x] 1.1 Confirm UX semantics (stop discards reply, latest only)
- [x] 1.2 Update `chat-ux` delta spec
- [x] 1.3 Run `openspec validate add-chat-stream-recovery-and-stop --strict --no-interactive`

## 2. Backend
- [x] 2.1 Add attach-only stream endpoint `GET /api/sessions/:id/stream`
- [x] 2.2 Add stop endpoint `POST /api/sessions/:id/stop`
- [x] 2.3 Ensure generation survives SSE disconnect and is cancelable via stop
- [x] 2.4 Do not persist assistant reply on stop (discard)
- [x] 2.5 Add backend tests (disconnect + attach; stop discard)

## 3. Frontend
- [x] 3.1 Add API client methods: `attachChatStream` + `stopSessionStream`
- [x] 3.2 Show Stop button while streaming; discard current reply on stop
- [x] 3.3 On reload, auto-attach to an in-flight stream when needed (best-effort)
- [x] 3.4 Add/adjust unit tests for stop + attach-on-reload

## 4. QA
- [ ] 4.1 Manual: start a long reply → refresh page → stream continues and finishes
- [ ] 4.2 Manual: start a long reply → click Stop → reply is discarded and generation halts
