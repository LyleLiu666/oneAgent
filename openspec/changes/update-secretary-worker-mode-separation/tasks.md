## 1. Spec Updates
- [x] 1.1 Update `chat-ux` to define secretary/full as **different conversation backends** (secretary vs worker)
- [x] 1.2 Update `app-shell-ux` to make `ui_mode` switch route + preserve per-mode sessions
- [x] 1.3 Update `system-secretary-orchestration` to enforce canonical secretary session + forbid cross-module session ids
- [x] 1.4 Update `system-error-surface` with stable `session_module_mismatch` error code
- [x] 1.5 Run `openspec validate update-secretary-worker-mode-separation --strict --no-interactive`

## 2. Backend Separation (after approval)
- [ ] 2.1 Add module guards for `/api/chat` (reject `module!=assistant`)
- [ ] 2.2 Make `/api/sessions/:id` assistant-only (reject `module!=assistant`) and add secretary-specific session read API
- [ ] 2.3 Make `/api/secretary/*` always use canonical secretary session; ignore/reject external `session_id`
- [ ] 2.4 Add tests for cross-module session rejection + canonical secretary session behavior

## 3. Frontend Separation (after approval)
- [ ] 3.1 Split store: `assistantChatStore` vs `secretaryChatStore` (or namespaced session ids + message lists)
- [ ] 3.2 Ensure `/` default entry respects `ui_mode` (secretary → `/secretary`, full → `/chat`)
- [ ] 3.3 Ensure secretary inbox/triage never sends arbitrary `session_id` (canonical only; do not invent ids client-side)
- [ ] 3.4 Add mode-switch tests to prevent session bleed between modes
