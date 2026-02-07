## 1. Spec
- [x] 1.1 Update `system-secretary-orchestration` delta requirements (continuity context + 80k budget + layered readonly search)
- [x] 1.2 Run `openspec validate update-secretary-su-fast-path --strict --no-interactive`

## 2. Backend
- [x] 2.1 Add triage carry context assembly (pending questions + semantic anchors + short follow-up hint)
- [x] 2.2 Inject carry context into SU triage prompt and keep `<= 80k` context budget with omission markers
- [x] 2.3 Add default home-root readonly search policy when session workspace is unset
- [x] 2.4 Add layered expansion (`home -> common-dev -> whitelist`) with timeout/visit limits
- [x] 2.5 Exclude hidden/system/high-noise directories by default
- [x] 2.6 Bind home root as readonly workspace for SU tool loop when session workspace is unset
- [x] 2.7 Persist triage `search_context` in `TriageRun` for traceability

## 3. Tests
- [x] 3.1 Follow-up prompt includes continuity anchors (no forgetful generic re-ask)
- [x] 3.2 Prompt context budget remains `<= 80k` and emits omission markers when compacted
- [x] 3.3 Prompt includes `default_readonly_search_root=<user_home>` for unbound workspace lookup
- [x] 3.4 Layered search auto-expands outside home when needed
- [x] 3.5 Hidden/system directories are excluded by default

## 4. Validation
- [x] 4.1 `go test ./...`
- [x] 4.2 `pnpm -C frontend test`
