## 1. Specs
- [x] 1.1 Update `system-secretary-orchestration` for permanent session + dual avatars + shared memory pull-sync
- [x] 1.2 Update `data-storage` for secretary memory SQLite persistence (time-range queries)
- [x] 1.3 Update `system-session-context-compression` to fix threshold at 80k and apply per secretary channel
- [x] 1.4 Update `chat-ux` to reflect “secretary is a permanent single conversation” (no session switching)
- [x] 1.5 Run `openspec validate add-secretary-dual-avatar-memory --strict --no-interactive`

## 2. Backend
- [x] 2.1 Add a secretary memory store (SQLite) with append + time-range query
- [x] 2.2 Add cursor-based pull-sync (last 10 + omitted count) and user-prompt injection payload builder
- [x] 2.3 Implement a permanent secretary session resolver (principal_id → canonical session_id)
- [x] 2.3.1 Record task attempt outcomes into Memory as SW (best-effort)
- [x] 2.4 Add SU/SW channel prompt assembly and enforce tool scopes (SU read-only; SW dispatch-only)
- [x] 2.5 Apply fixed 80k context compression to SU/SW channels (reuse existing compression implementation)

## 3. Tests (TDD)
- [x] 3.1 Memory: append + query by time window
- [x] 3.2 Pull-sync: 0/3/23 unsynced cases (last 10 + omitted count + cursor update)
- [x] 3.3 Permanent session: restart/reload returns same secretary session_id
- [x] 3.4 Compression: over-80k triggers summary; under threshold does not; failure degrades safely

## 4. Frontend (best-effort)
- [x] 4.1 Secretary route uses the permanent secretary conversation (no session switching UX)
- [x] 4.2 Ensure secretary mode does not surface worker noise by default; advanced details remain discoverable
