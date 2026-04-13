# Change: Externalize The Shared Agent Runtime Kernel To agentsdk

## Why
`oneAgent` 目前在 `backend/internal/llm`、chat tool loop、secretary XML loop 里维护了一套和外部 `agentsdk` 高度重叠的运行内核逻辑。这样会带来两个问题：

1. 通用能力会双维护并逐渐漂移，尤其是 prompt cache、tool loop 和坏参数容错。
2. 用户已经明确希望只维护一套运行内核，而不是在 `oneAgent` 里再复制一份。

外部 `agentsdk v0.5.2` 已经补齐本次迁移最关键的公共能力：
- `volatile` / `force_cacheable` 提示缓存语义
- 统一 `BuildPromptCacheKey(...)`
- cache 字段 4xx 的自动降级重试与回调
- 坏 tool arguments 的共享 best-effort / strict 规则

因此现在具备了把“共享运行内核”外置回 `agentsdk` 的基础。

## What Changes
- 在 `backend` 中直接依赖 `agentsdk v0.5.2`，并在本地开发阶段通过 `replace` 指向 `/Users/liu_y/code/goProject/AgentAll/agentsdk`
- 第一期先替换最核心、最容易漂移的公共内核：
  - prompt cache key 生成
  - prompt cache 稳定/易变消息语义与选择逻辑
  - 坏 tool arguments 容错
- 保留 `oneAgent` 作为宿主层，继续负责：
  - sessions / persistence
  - permissions / settings
  - task queue / secretary orchestration
  - SSE / trace / llm log / UI
- 为后续 chat / secretary loop 切换到 `agentsdk toolloop/jsonprotocol/xmlprotocol` 预留薄适配层边界

## Impact
- Affected specs:
  - `system-agent-factory`
  - `system-llm-prompt-caching`
  - `system-toolcalling-reliability`
- Affected code:
  - `/Users/liu_y/code/goProject/oneAgent/backend/go.mod`
  - `/Users/liu_y/code/goProject/oneAgent/backend/internal/llm/*`
  - 后续阶段会继续影响 chat / secretary runtime adapter
