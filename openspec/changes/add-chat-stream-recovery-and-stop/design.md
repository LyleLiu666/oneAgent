# Design: Add chat stream recovery and stop

## Goals
- 用户可以中止正在生成的 assistant 输出，避免“卡死感”与无意义等待。
- 流式连接出现可恢复的中断时，UI/后端能 best-effort 给出恢复或明确提示。
- Stop/恢复不引入 `stream=false` 依赖；依旧以流式为主，避免超时。

## Non-goals
- 不承诺“从中断点继续生成”的严格语义（多数上游不支持），优先保证用户感知与数据不丢。

## Approach (high-level)
1) 为每个 streaming chat 请求生成一个可寻址的“流实例 id”（或复用现有 streamId/sessionId + step）。
2) 前端在 streaming 状态展示 Stop 按钮，触发后端 stop（HTTP endpoint 或 SSE control channel）。
3) 后端收到 stop 后：
   - 取消上游 LLM 请求（context cancel）
   - 尽快发送 final/done 事件
   - best-effort 写入已生成的 assistant 内容（避免丢失）
4) 对短暂网络错误：
   - 仅做 best-effort：提示可重试/自动重连（避免 silent hang）

## Notes
- SSE/HTTP streaming 不应使用 `http.Client.Timeout` 作为整请求超时，否则会在长流中断（表现为 `context deadline exceeded (Client.Timeout ...)`）。

