## 1. Implementation
- [x] 1.1 Add `trash_file` tool id + registry mount + safety metadata
- [x] 1.2 Implement `trash_file` (move-to-trash + result payload)
- [x] 1.3 Implement trash retention cleanup (7 days, best-effort)
- [x] 1.4 Wire periodic cleanup (startup + interval loop per workspaceRoot, best-effort)
- [x] 1.5 Add unit tests for `trash_file` and cleanup behavior

## 2. Validation
- [x] 2.1 `cd backend && go test ./...`
- [x] 2.2 `openspec validate add-trash-file-tool --strict --no-interactive`
