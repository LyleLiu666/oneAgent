# Change: Add session context compression verification（确认“自动压缩”真的生效）

## Why
目前后端已实现“会话过长自动压缩”（`backend/internal/handler/session_compress.go`），但存在两个问题：
- **很少触发 / 难以确认**：缺少可操作的验证路径与回归测试，导致开发者/用户无法确信它是否生效。
- **不可回归**：未来对 prompt assembling、KV cache、session store 的改动可能悄悄破坏压缩逻辑，却没有测试兜底。

这会直接影响 oneAgent 的“长跑能力”：长任务/长会话更容易失忆、退化、或成本飙升。

## What Changes
- 新增一个专门的 OpenSpec capability：`system-session-context-compression`，把压缩的触发条件、保留策略、输出格式、失败降级与可观测性写清楚。
- 补齐回归测试（不依赖真实 LLM）验证：
  - 达到阈值时会触发压缩并重建 prompt
  - 压缩会写入 `【会话压缩】` 摘要消息到 session store
  - 摘要失败时的 fallback 行为可预期且不会误删当前用户消息
- （可选）把阈值做成可配置（env），方便在 dev 环境低成本触发验证。

## Impact
- Affected specs:
  - `system-session-context-compression`（NEW）
- Affected code (expected):
  - `backend/internal/handler/session_compress.go`
  - `backend/internal/handler/chat.go`（trace/调用点）
  - 新增：`backend/internal/handler/session_compress_test.go`（或等价测试）

