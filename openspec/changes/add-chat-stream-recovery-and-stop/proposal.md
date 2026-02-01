# Change: Add chat stream recovery and stop

## Why
当前长时间的 LLM 流式输出有两个显著痛点：
- 用户侧：没有明显的“正在进行/可中止”信号时，容易误以为卡死。
- 系统侧：当网络抖动、代理/HTTP2 连接异常、或上游耗时较长时，流容易被中断，导致用户体验差、并可能造成会话落库不完整。

## What Changes
- 在聊天 UI 中，为正在流式输出的 assistant 消息提供显式的 “Stop/停止生成” 能力（best-effort）。
- 后端在收到 stop 后，应尽快取消上游请求并收敛流状态：
  - 结束当前流（发送 final/done）
  - best-effort 持久化已生成的内容（避免丢失）
  - 标注停止原因（用户主动停止 vs 超时/错误）
- 对可恢复的中断场景（例如短暂网络抖动）提供 best-effort 的恢复策略（例如重试/重新连接），避免用户频繁手动刷新。

## Impact
- Affected specs: chat-ux
- Affected code: backend `/api/chat` SSE streaming, llm http client, frontend streaming UI

