# Design: Secretary inbox + triage

本文是 `add-secretary-inbox-and-triage` 的技术设计补充（对应 `docs/secretary-multi-worker-design.md` 的最小闭环落地版本）。

## Goals
- 支持“微信式”连续发送：append-only，不阻塞
- 每条消息先 quick ack（LLM 生成、短句、无工具；落盘可回溯）
- 支持一次 triage 覆盖多条未处理消息：低噪声汇报 + **默认自动派工**
- 保证幂等：同一批输入不重复派工
- 支持 workspace allocator：同一 workspace 串行；不相关工作尽量分配到不同 workspace 并行（必要时自动创建新 workspace）
- 保持后向兼容：full mode chat 仍走 `/api/chat`

## Non-Goals (v1)
- 不做 workstreams 卡片 UI（先用“汇报消息 + 任务交付卡片”完成闭环）
- 不做同一 workspace 真并行写（worktree 并行 + 合并流程后置）

## API Sketch

### 1) Append inbox message
`POST /api/secretary/inbox/messages`

Request:
```json
{
  "session_id": "optional",
  "content": "user message",
  "workspace": "optional"
}
```

Behavior:
- 追加一条 session message（role=user,type=text）
- 调用 LLM 生成 quick ack（短句、无工具、不做分析/派工；建议点出/复述 1 个关键信息以证明“看过并记下”）
- 追加一条 assistant ack message（建议设置 `parent_id = <user_message_id>` 以便回溯关联）

Response:
```json
{
  "session_id": "resolved",
  "message_id": 123,
  "ack_message_id": 124,
  "ack_text": "..."
}
```

### 2) Triage
`POST /api/secretary/triage`

Request:
```json
{
  "session_id": "required",
  "cursor_message_id": 120
}
```

Behavior (best-effort):
- 读取 `cursor_message_id` 之后的 user messages
- 调用 LLM 生成结构化 triage 结果：
  - `summary_message`（给用户的低噪声汇报）
  - `tasks[]`（每条包含 title/prompt/workspace 等；系统默认按此自动派工）
  - `questions[]`（需要用户确认的点）
- 创建并 enqueue tasks（worker = Task Queue）
- 追加 1 条 assistant 汇报消息到 session messages
- 更新 cursor，并记录这次 triage 的 input range → created_task_ids（用于幂等）

Response:
```json
{
  "summary_message": "...",
  "cursor_message_id": 123,
  "created_task_ids": ["..."],
  "questions": ["..."],
  "workspaces_created": ["..."]
}
```

### 3) State
`GET /api/secretary/state?session_id=...`

Response:
- 当前 cursor、最近 triage 记录、与该 session 关联的 created_task_ids（best-effort）

## Storage & Idempotency

v1 建议复用 `session.Metadata["secretary"]`，避免引入新 store：
- `cursor_message_id`: uint
- `triage_runs`: 最近 N 次（input range hash / max_id → created_task_ids）

幂等判定建议：
- `input_range = (prev_cursor, max_message_id)` 或 `input_ids_hash`
- 若 `input_range` 已存在，则直接返回记录，不再次创建 task

## Concurrency
- triage 需要互斥：同一 session 同时只能有一个 triage 在跑（可用 sessionstore lock + in-memory guard best-effort）
- append 不互斥：允许用户持续追加消息

## Workspace allocation (allocator)

目标：在不破坏现有 “workspace 是工具写入边界” 的前提下，让秘书能在以下场景自动分配 workspace：
- 同一 repo 的改动任务：落到同一 workspace（由 Task Queue 串行保障）
- 与 repo 无关的泛化任务（报告/文章/资料整理）：分配到一个独立 workspace（可自动创建目录），从而与代码仓库隔离并获得并行度

建议策略（best-effort，按优先级）：
1) **显式 workspace**：请求里带 `workspace` → 使用该 workspace
2) **会话 workspace**：session metadata 已绑定 workspace → 使用该 workspace
3) **可自动创建**：若任务不依赖既有 repo（由 triage 判断），则在一个“workspace pool root”下创建新目录作为 workspace，并返回路径给用户
4) **需要用户确认**：若任务看起来需要某个 repo，但系统无法推断 → `questions[]` 询问用户选择/指定 workspace

workspace pool root 必须 (MUST) 支持通过 `ONEAGENT_WORKSPACE_POOL_DIR` 覆盖；默认值为 `ONEAGENT_HOME/.oneagent/workspaces`，并在启动或首次使用时创建该目录（best-effort）。

## Frontend Integration Plan
- secretary mode 下：
  - send -> 调用 inbox append API（并立即在 UI 展示 user 消息）
  - debounce -> 调用 triage API（收到 summary 后插入 assistant 消息）
- full mode 下：
  - 保持现状：`/api/chat` SSE 流式对话 + tool calling
