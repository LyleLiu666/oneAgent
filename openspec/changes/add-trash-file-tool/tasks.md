## 1. Implementation
- [ ] 1.1 Add `trash_file` tool id + registry mount + safety metadata
- [ ] 1.2 Implement `trash_file` (move-to-trash + result payload)
- [ ] 1.3 Implement trash retention cleanup (7 days, best-effort)
- [ ] 1.4 Wire periodic cleanup (startup + interval loop per workspaceRoot, best-effort)
- [ ] 1.5 Add unit tests for `trash_file` and cleanup behavior

## 2. Validation
- [ ] 2.1 `go test ./...`
- [ ] 2.2 `openspec validate add-trash-file-tool --strict --no-interactive`

