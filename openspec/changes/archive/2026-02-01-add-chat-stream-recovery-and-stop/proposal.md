# Change: Add chat stream recovery and stop

## Why
当前长时间的 LLM 流式输出有两个显著痛点：
- 用户侧：没有明显的“正在进行/可中止”信号时，容易误以为卡死。
- 系统侧：当网络抖动、代理/HTTP2 连接异常、或上游耗时较长时，流容易被中断，导致用户体验差、并可能造成会话落库不完整。

## What Changes
- 在聊天 UI 中，为正在流式输出的 assistant 消息提供显式的 “Stop/停止生成” 能力（best-effort）。
- 后端在收到 stop 后，应尽快取消上游请求并收敛流状态（best-effort）：
  - 结束当前流（通知所有订阅者停止）
  - **不持久化本次 assistant 回复**（discard；符合用户预期“停止就是不要这次回复”）
- 对刷新/断线场景提供 best-effort 的“重新 attach”策略：用户刷新页面后可继续观察同一 session 的进行中流式输出，避免会话被动中断。

## Impact
- Affected specs: chat-ux
- Affected code: backend chat SSE (attach/stop), frontend streaming lifecycle (stop button + attach-on-reload)
