# Change: Separate secretary vs worker conversation modes (UI + backend)

## Why
当前“秘书模式 / 完整模式”的语义被混用：有时它只是 UI 降噪，有时它又被当作“与秘书对话”的模式。由于前后端共用同一套 session / message list / session_id，用户可能在不知情的情况下把消息发给了“另一种 agent”（例如在秘书界面里落到 worker chat session，或在完整模式里误用秘书 session），导致：
- 对话对象不稳定（用户以为在跟秘书聊，实际跟 worker 聊；反之亦然）
- 留痕与可追溯被破坏（一个 session 内混入两类 agent 的消息/协议）
- debug 成本上升（用户“明明有留痕”但 agent 声称看不到/对不上）

## What Changes
- **概念澄清（强约束）**
  - **完整模式**：用户与 **Worker Chat（可 tool-calling 的交互式主 agent）** 对话；这是“直接做事”的会话。
  - **秘书模式**：用户与 **Secretary（中层管理者）** 对话；秘书只做归并、解释、派工与进度汇报，不直接执行工具。
  - **术语澄清**：本文的 “Worker Chat” 指交互式对话 agent；Task Queue 里的后台执行单元仍称为 “background task/worker task”，避免概念混淆。
- **前端分离**
  - `/chat` 与 `/secretary` 两套页面/状态分离：各自维护独立的 `session_id` 与 message list，不再共用一个 `chatStore.currentSessionId`。
  - `ui_mode`（secretary/full）切换时，显式切换“对话对象”并跳转到对应路由（并保留两边最近会话，避免互相覆盖）。
  - 秘书模式请求不再携带任意 `session_id`（由后端返回 canonical secretary session；防止客户端自造/串台）。
- **后端分离**
  - `/api/chat` 与 `/api/sessions/*` 仅允许操作 `module=assistant` 的会话；遇到其它 module 返回稳定错误码并提示正确入口。
  - `/api/secretary/*` 仅允许操作 `module=secretary` 的会话；并且必须使用该用户的 canonical secretary session（**忽略**外部传入的 `session_id`，best-effort）以避免串台。
  - 为秘书模式提供独立的“读取会话/消息”接口（避免复用 `/api/sessions/:id` 造成语义混乱）。

## Impact
- Affected specs:
  - `chat-ux`（模式定义与路由/交互）
  - `app-shell-ux`（ui_mode 与默认入口/切换行为）
  - `system-secretary-orchestration`（session 边界与 API 约束）
  - `system-error-surface`（新增错误码：session/module mismatch）
- Affected code (expected):
  - Frontend routing + stores（拆分会话状态）
  - Backend handlers：`handler/chat.go`、`handler/secretary.go`、`sessionstore`（或 handler 层校验）

## Risks / Mitigations
- **BREAKING 行为**：历史上混入两类消息的 session 可能无法“自动纠正”。
  - 规避：不做历史迁移；只从现在开始阻止新混入，并给出可操作的错误提示与一键恢复（例如切换到正确模式/正确路由）。
- **用户心智迁移**：从“同一 Chat 里切模式”变为“对话对象切换”。
  - 规避：在切换入口处使用低噪声说明（一次性），并保留两边最近会话，避免“丢对话”。
