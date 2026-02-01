# Change: Add secretary inbox + triage (WeChat-style multi-message, manager → workers)

## Why
当前秘书模式仍主要是“单条消息 → 单次对话回复”的交互模型：用户无法像微信一样连续发多条消息后等秘书统一理解与安排；同时 chat 通道会产生 tool_call/tool_result 噪声，使秘书更像一个亲自干活的 worker。

oneAgent 的北极星要求“秘书 = 编排器”：承接多线程意图、拆解为工作线，并调度多个 work-style workers（Task Queue / subagent）并行交付（见 `openspec/project.md`、`openspec/roadmap-secretary-first.md`、`docs/secretary-multi-worker-design.md`）。

## What Changes
- 新增“收件箱（append-only）+ 归并/派工（triage）”能力：
  - 秘书模式下允许用户连续发送多条消息，不因生成中而阻塞
  - 每条消息都会获得一个快速确认（quick ack）：**由 LLM 生成**（短句、无工具、不做分析/派工），并作为一条 assistant message 落盘，保证可回溯
  - 在短暂静默窗口后，秘书对“自上次 triage 以来的消息集合”做一次统一归并，并输出低噪声汇报
  - 秘书**默认自动**将可执行工作派发为后台 tasks（workers），并保证幂等（相同输入不重复派工）
  - 当用户未指定 workspace 且工作不依赖既有 repo 时，系统可自动创建一个新文件夹作为 workspace（用于并行与隔离），并在汇报中告知路径（best-effort）
- 新增后端 Secretary Orchestrator API（最小闭环）：
  - `POST /api/secretary/inbox/messages`（append-only + LLM quick ack，无工具）
  - `POST /api/secretary/triage`（归并/派工/写入汇报）
  - `GET /api/secretary/state`（恢复）

## Impact
- Affected specs:
  - `chat-ux`（新增 secretary multi-message 交互要求）
  - `system-secretary-orchestration`（新增 capability：inbox/triage/dispatch/idempotency）
- Affected code (expected):
  - Backend: `backend/internal/handler/*`, `backend/internal/sessionstore/*`, `backend/internal/taskqueue/*`
  - Frontend: `frontend/src/components/ChatBox.vue`（secretary mode composer flow）
- Related docs:
  - `docs/secretary-multi-worker-design.md`

## Out of Scope (for this change)
- Workstreams 卡片 UI（可在后续 change 补齐）
- 同一 workspace 真并行写（worktree 并行 + 合并流程，建议独立评审）
