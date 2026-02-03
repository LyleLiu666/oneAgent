## Context
当前实现把“秘书/完整”同时当作 UI 降噪开关与对话对象选择，前端共用 `chatStore.currentSessionId`，后端 session API 不校验 `session.Module`。结果是：
- `/api/chat` 可能写入到 `module=secretary` 的 session
- `/api/secretary/*` 可能写入到 `module=assistant` 的 session
- `/api/sessions/:id` 读取语义不明确（list 是 assistant-only，get 却是 any-module）

## Goals
- 明确：**秘书模式=与秘书对话**；**完整模式=与 worker 对话**
- 强约束：不同模式的 session / message list 不互相污染（前端与后端双重保障）
- 维持：留痕 append-only（不做历史重写/迁移）
- 可恢复：发生 module mismatch 时给出可操作的错误码与下一步

## Non-Goals
- 不对历史混杂 session 做自动拆分/迁移（风险高且破坏可追溯）
- 不改变 Task Queue / Worker 的执行语义（仅改变入口与会话边界）

## Key Decisions
### 0) Glossary (avoid name confusion)
- **Secretary**：用户在“秘书模式”中对话的中层管理者（`module=secretary`）
- **Worker Chat**：用户在“完整模式”中对话的交互式主 agent（`module=assistant`）
- **Worker Task / Background Task**：Task Queue 派发的后台执行单元（task attempts）

### 1) Session modules are hard boundaries
- worker chat session (Worker Chat): `module=assistant`
- secretary session: `module=secretary`
- secretary internal planner session: `module=secretary_sw`（仅后端内部使用）

### 2) Canonical secretary session per principal
后端以 Settings 中的 `secretary_session_id` 作为该用户唯一的“秘书会话 ID”（best-effort stable）。
`/api/secretary/*` 默认使用 canonical session；客户端不应自造/传入其它 session_id。

为了“放权与信任 + 可追溯”，本变更优先采取 **容错式修复**：当客户端误传 `session_id` 时，秘书相关 API 以 canonical session 为准（best-effort），避免把用户对话写到“另一条线”。

### 3) API boundaries
- Worker chat API（assistant-only）：
  - `/api/chat`
  - `/api/sessions`、`/api/sessions/:id`、`/api/sessions/:id/stream|stop|truncate|delete`
- Secretary API（secretary-only）：
  - `/api/secretary/inbox/messages`
  - `/api/secretary/triage`
  - `/api/secretary/state`
  - + 新增：`/api/secretary/session`（读取 canonical secretary session + messages）

### 4) Error surface for mismatch
当 API 发现 `session.Module != expectedModule`：
- 返回 HTTP 409 + `code=session_module_mismatch`
- `error`：用户安全的中文提示
- `hint`：建议切换到正确模式/路由
- `request_id`：用于 trace/log

### 5) Frontend state separation
前端需要把“模式”当作两条不同的对话线：
- `/secretary` 只读写 secretary session（canonical）
- `/chat` 只读写 assistant sessions
切换模式时要保留两边最近 session，不共用同一个 `currentSessionId`。
