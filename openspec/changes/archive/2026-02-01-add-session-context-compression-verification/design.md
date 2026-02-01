## Context
会话压缩是“长会话稳定性”的核心机制之一，必须做到：
- 触发条件明确
- 压缩输出格式稳定（便于后续引用/缓存）
- 压缩失败可降级且可观测
- 有可回归测试，避免改动后静默失效

## Goals / Non-Goals
- Goals
  - 提供可回归的测试覆盖压缩 happy path + fallback
  - 提供开发者可确认的 observability 信号（trace/日志）
  - （可选）提供一个 dev 级别的低阈值开关，便于手工验证
- Non-Goals
  - 不在本变更中调整压缩 prompt 的写作质量（属于 prompt/模型效果范畴）
  - 不在本变更中引入复杂的 token 级计数器（先用现有 rune 近似；后续再迭代）

## Decisions
- Decision: 测试不依赖真实 LLM
  - 使用 fake `llm.Client` 返回固定摘要字符串
  - 断言 session store 的 messages 被 Replace 为：`【会话压缩】...` + tail messages
- Decision: 可配置阈值（可选）
  - 引入 env：`ONEAGENT_SESSION_COMPRESSION_MAX_CONTEXT_RUNES`（仅用于触发阈值）
  - 默认仍保持当前常量值，避免行为改变

## Risks / Trade-offs
- 过度可配置会引入“线上被误配”风险：默认值必须稳定；配置仅作为 override，并在 doctor/日志中可见（best-effort）。

## Migration Plan
1) 写 OpenSpec capability + scenarios（本次已完成）
2) 写单测覆盖：触发/不触发/摘要失败 fallback
3) （可选）加入 env override 并在测试中使用更低阈值
4) `go test ./...` + `openspec validate ...`

## Open Questions
- 阈值是否要从 rune 近似升级为 token 近似？（可能需要 tokenizer，引入依赖与一致性问题）

