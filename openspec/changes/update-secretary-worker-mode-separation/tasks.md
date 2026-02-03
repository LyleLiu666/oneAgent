## 1. Spec Updates
- [x] 1.1 Update `chat-ux` to define secretary/full as **different conversation backends** (secretary vs worker)
- [x] 1.2 Update `app-shell-ux` to make `ui_mode` switch route + preserve per-mode sessions
- [x] 1.3 Update `system-secretary-orchestration` to enforce canonical secretary session + forbid cross-module session ids
- [x] 1.4 Update `system-error-surface` with stable `session_module_mismatch` error code
- [x] 1.5 Run `openspec validate update-secretary-worker-mode-separation --strict --no-interactive`

## 2. Backend Separation (after approval)
- [x] 2.1 Add module guards for `/api/chat` (reject `module!=assistant`)
- [x] 2.2 Make `/api/sessions/:id` assistant-only (reject `module!=assistant`) and add secretary-specific session read API
- [x] 2.3 Make `/api/secretary/*` always use canonical secretary session; ignore/reject external `session_id`
- [x] 2.4 Add tests for cross-module session rejection + canonical secretary session behavior

## 3. Frontend Separation (after approval)
- [x] 3.1 Split store: `assistantChatStore` vs `secretaryChatStore` (or namespaced session ids + message lists)
- [x] 3.2 Ensure `/` default entry respects `ui_mode` (secretary → `/secretary`, full → `/chat`)
- [x] 3.3 Ensure secretary inbox/triage never sends arbitrary `session_id` (canonical only; do not invent ids client-side)
- [x] 3.4 Add mode-switch tests to prevent session bleed between modes

## 4. Secretary Agentic Upgrade (spec first)
- [x] 4.1 Update `system-secretary-orchestration` delta: triage can tool-call but must be read-only to workspace
- [x] 4.2 Add `system-tool-permissions` delta: secretary default filesystem policy is read-only (no write/delete/modify)
- [x] 4.3 Add `system-toolcalling-reliability` delta: tool errors feed back to agent loop for self-heal (bounded)
- [x] 4.4 Add `system-task-queue` delta: Outcome Observer output parse resilience + auto-retry (bounded)
- [x] 4.5 Re-run `openspec validate update-secretary-worker-mode-separation --strict --no-interactive`

## 5. Reliability Implementation (after approval)
- [ ] 5.1 Implement Outcome Observer self-heal retry for invalid XML/JSON output (tests first)
- [ ] 5.2 Ensure tool-loop feeds provider/tool errors back to LLM for retry (worker + secretary)
- [ ] 5.3 Ensure secretary uses restricted tool permissions (read-only filesystem) and can delegate to worker tasks when needed
