## 1. Specs
- [x] 1.1 Update `chat-ux` spec: recovery becomes a chat conversation (relay + reply-to-resume) + troubleshoot opens trace inline first
- [x] 1.2 Update `system-secretary-orchestration` spec: secretary mediates recovery as an agentic, traceable conversation (tools allowed but no file mutation; engineer errors self-heal first)
- [x] 1.3 Run `openspec validate update-secretary-recovery-conversation --strict --no-interactive`

## 2. Frontend (secretary mode)
- [x] 2.1 Detect “needs-attention” task transitions and append a secretary relay message into chat (no history replay)
- [x] 2.2 Add secretary recovery inbox: keep a focused current item, show all items with concrete details, allow switching (no “只报数量”)
- [x] 2.3 On user reply to the current ask, call `resumeTask(id, { review_notes })` and append a chat receipt (traceable)
- [x] 2.4 Improve troubleshoot action: open trace modal when available; fallback to full mode `/tasks`
- [x] 2.5 Fold direct-to-user artifacts/actions behind “更多/展开” in secretary mode; keep advanced access without clutter
- [x] 2.6 Tests: relay message formatting + queue + reply triggers resume once + progressive disclosure best-effort
  - [x] 2.6.1 When latest attempt transitions running→failed, Chat appends 1 assistant relay message (no history replay)
  - [x] 2.6.2 When N>=2 tasks need attention, Chat surfaces multiple recovery items with concrete reason/next-step/questions, and focuses one item by default
  - [x] 2.6.3 Replying once triggers `resumeTask(taskId, { review_notes })` exactly once for the focused item, and keeps binding correct when switching
  - [x] 2.6.4 After the recovery inbox drains (or user defers), normal secretary sending resumes
  - [x] 2.6.5 “排障” opens trace modal if `trace_log_path` exists; otherwise navigates to `/tasks` and switches to full mode
  - [x] 2.6.6 Artifacts buttons are not visible by default in secretary mode; become visible after “展开/更多”

## 3. Backend (optional best-effort)
- [x] 3.1 Add a lightweight server-side “secretary recovery state” to persist active binding across refresh
- [x] 3.2 Ensure resume events record `review_notes` source as `secretary-recovery` (best-effort)
