## 1. Backend
- [ ] 1.1 Add stop signal path for `/api/chat` streaming sessions
- [ ] 1.2 Cancel upstream LLM request on stop
- [ ] 1.3 Persist partial assistant message (best-effort)
- [ ] 1.4 Emit explicit final status (stopped vs error)
- [ ] 1.5 Add tests for stop + persistence behavior

## 2. Frontend
- [ ] 2.1 Show “Stop/停止生成” while assistant message is streaming
- [ ] 2.2 Show final state when stopped (best-effort)
- [ ] 2.3 Best-effort retry/reconnect UX for recoverable stream failures
- [ ] 2.4 Add unit/e2e tests for stop + reconnect UI

## 3. QA
- [ ] 3.1 Manual test: long response, stop mid-stream, verify partial content retained
- [ ] 3.2 Manual test: simulate stream interruption, verify UI feedback + recovery behavior

