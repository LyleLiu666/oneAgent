## 1. Spec & Design (planning first)
- [x] 1.1 Create new capability spec `system-session-context-compression`
- [x] 1.2 Define compression scenarios (trigger + fallback + non-trigger)
- [x] 1.3 Decide whether to add env override for compression threshold

## 2. Implementation (after proposal approval)
- [x] 2.1 Add unit tests for `compressSessionIfNeeded` (fake llm client)
- [x] 2.2 Add regression test for fallback path (summary error)
- [x] 2.3 (Optional) Add env override: `ONEAGENT_SESSION_COMPRESSION_MAX_CONTEXT_RUNES`
- [ ] 2.4 Ensure trace messages remain low-noise but confirmable (best-effort)

## 3. Tests & Validation
- [x] 3.1 `go test ./...` (backend)
- [x] 3.2 `openspec validate add-session-context-compression-verification --strict --no-interactive`
