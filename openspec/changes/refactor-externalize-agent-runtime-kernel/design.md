## Context
`oneAgent` 的愿景是做一个工业级 agent 平台，但短期优先级是把核心 agent loop 做得更稳、更快、更低损耗。对这个目标来说，真正应该共享的不是 UI 或任务编排，而是运行内核本身：

- prompt cache 语义
- cache key
- provider cache 兼容降级
- tool loop 的参数容错与协议恢复

这些能力已经在外部 `agentsdk` 中独立沉淀。继续在 `oneAgent` 内复制一份，会直接增加维护成本，并降低不同宿主之间的一致性。

## Goals
- 让共享运行内核以 `agentsdk v0.5.2` 为准
- 让 `oneAgent` 只保留宿主层职责，不再维护重复规则
- 用契约测试锁定与 SDK 一致的行为

## Non-Goals
- 本次不删除 `oneAgent` 全部 backend 代码
- 本次不重写 session store、secretary、task queue、permissions
- 本次不强行一次性迁移所有 chat / secretary loop

## Boundary
保留在 `oneAgent` 的宿主层能力：
- session/message 持久化
- tool policy / permissions
- settings / model resolution
- task queue / secretary orchestration
- SSE / trace / llm log / UI

迁移到 `agentsdk` 的共享运行内核能力：
- prompt cache behavior (`volatile` / `force_cacheable`)
- prompt cache key helper
- provider prompt cache 兼容语义
- malformed tool arguments recovery
- 后续的 JSON/XML tool loop engine

## Phase Plan
### Phase 1
先替换“公共规则”而不是整条执行链：
- `BuildPromptCacheKey` 直接调用 SDK helper
- cacheable message selection 对齐 SDK
- tool arguments sanitize 直接调用 SDK shared implementation
- 增加和 SDK 契约一致的测试

这样可以先把最容易漂移的基础规则收敛到同一份实现上，同时不打断现有 session/SSE/trace 流程。

### Phase 2
在 `oneAgent` 中增加薄适配层，把 chat / secretary 的 loop 切到 SDK engine：
- `chat.go` 使用 shared JSON tool loop
- `secretary/orchestrator.go` 使用 shared XML loop
- 由适配层把 SDK 事件写回 `oneAgent` 的 session、trace、SSE、llm_calls

## Testing
最小验收必须覆盖：
- 同一会话只改 turn context，cache key 不变
- summary 改了，cache key 变化
- tool schema / prefix version 改了，cache key 变化
- 普通 4xx 不误触发 cache 降级重试
- 坏 tool args 至少保留 `_raw`
