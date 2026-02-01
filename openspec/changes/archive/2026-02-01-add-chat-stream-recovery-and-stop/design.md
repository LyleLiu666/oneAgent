# Design: Add chat stream recovery and stop

## Goals
- 用户可以中止正在生成的 assistant 输出，避免“卡死感”与无意义等待。
- 流式连接出现可恢复的中断时，UI/后端能 best-effort 给出恢复或明确提示。
- Stop/恢复不引入 `stream=false` 依赖；依旧以流式为主，避免超时。

## Non-goals
- 不承诺“从中断点继续生成”的严格语义（多数上游不支持），优先保证用户感知与数据不丢。

## Approach (high-level)
1) 后端以 `session_id` 为粒度维护一个 `StreamBroadcaster`（内存态）：
   - 生成过程使用 `context.WithCancel(context.Background())`，不绑定单个 SSE 连接。
   - 广播 `msg/trace/usage/error/done` 等事件给所有订阅者（多标签页）。
   - 额外维护“进行中的 assistant text”快照，用于 reload 后补齐（best-effort）。
2) 前端在 streaming 状态展示 Stop 按钮：
   - 立即中止本地 SSE 读取（AbortController）
   - 调用后端 stop endpoint（见 API），并丢弃本次 assistant 回复的 UI 气泡（discard）
3) 后端 stop：
   - 将该 session 标记为 canceled，并触发 cancel func（best-effort 取消上游请求）
   - 广播 `canceled` 事件给所有订阅者
   - **不落盘本次 assistant 回复**（discard）
4) Reload/re-attach：
   - `GET /api/sessions/:id/stream` 仅用于 attach（不触发新生成）
   - 先发送快照（start+delta），再继续推送后续事件（best-effort）

## Notes
- SSE/HTTP streaming 不应使用 `http.Client.Timeout` 作为整请求超时，否则会在长流中断（表现为 `context deadline exceeded (Client.Timeout ...)`）。
