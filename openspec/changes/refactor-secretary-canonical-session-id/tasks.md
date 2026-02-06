## 1. Spec
- [ ] 1.1 Update `system-secretary-orchestration` canonical session requirement (deterministic id + migration guidance)

## 2. Backend
- [ ] 2.1 Implement deterministic canonical SU session id derivation (filesystem-safe; per principal; local default `secretary`)
- [ ] 2.2 Implement best-effort migration from legacy UUID canonical SU/SW sessions to deterministic ids (preserve message ids/cursors)
- [ ] 2.3 Keep reset semantics: delete SU only, keep SW

## 3. Tests
- [ ] 3.1 Unit tests: canonical secretary session id deterministic per principal
- [ ] 3.2 Integration-ish tests: migration from legacy UUID session to deterministic id preserves messages and next_message_id

## 4. Validation
- [ ] 4.1 `openspec validate refactor-secretary-canonical-session-id --strict --no-interactive`
- [ ] 4.2 `go test ./...`

