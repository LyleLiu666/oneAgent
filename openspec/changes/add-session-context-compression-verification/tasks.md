## 1. Spec & Design (planning first)
- [x] 1.1 Create new capability spec `system-session-context-compression`
- [x] 1.2 Define compression scenarios (trigger + fallback + non-trigger)
- [ ] 1.3 Decide whether to add env override for compression threshold

## 2. Implementation (after proposal approval)
- [ ] 2.1 Add unit tests for `compressSessionIfNeeded` (fake llm client)
- [ ] 2.2 Add regression test for fallback path (summary error)
- [ ] 2.3 (Optional) Add env override: `ONEAGENT_SESSION_COMPRESSION_MAX_CONTEXT_RUNES`
- [ ] 2.4 Ensure trace messages remain low-noise but confirmable (best-effort)

## 3. Tests & Validation
- [ ] 3.1 `go test ./...` (backend)
- [ ] 3.2 `openspec validate add-session-context-compression-verification --strict --no-interactive`

