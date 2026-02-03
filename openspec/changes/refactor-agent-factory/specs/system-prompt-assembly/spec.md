# system-prompt-assembly Spec Delta

## ADDED Requirements

### Requirement: All agents MUST use the same prompt assembler via Agent Factory (best-effort)
系统必须 (MUST) 通过 Agent Factory（或等价统一入口）复用同一套 prompt assembler 来构建稳定前缀（stable prefix）（best-effort），避免在不同 agent（worker/secretary/subagent）中出现“各自字符串拼接/各自注入约束”的重复实现，导致 KV-cache 与可靠性策略无法一致生效。

#### Scenario: Worker and secretary share the same stable prefix assembly rules
- **GIVEN** Worker 与 Secretary 在同一轮使用相同的 tool protocol 与相同的 tool manuals 资产集合（best-effort）
- **WHEN** 系统为两者构建 stable prefix（best-effort）
- **THEN** stable prefix 的关键约束与 tool manuals 规则一致（best-effort）
- **AND** 不会因为 agent 类型不同而漏注入/重复注入某些稳定约束（best-effort）

### Requirement: Volatile snapshots MUST be injected as TurnContext, not merged into stable prefix (best-effort)
系统必须 (MUST) 将诸如 tasks snapshot / observer snapshot / skills recall summary 等“易变快照”作为 TurnContext（volatile）注入（best-effort），不得在每次请求时把这些内容拼接进 system prompt（否则会破坏 stable prefix 的可缓存性与可解释性）。

#### Scenario: Task snapshot does not pollute stable prefix
- **GIVEN** 同一会话内两轮请求的稳定配置一致（best-effort）
- **AND** 两轮的 tasks snapshot 不同（例如任务状态变化；best-effort）
- **WHEN** 系统构建并发送 prompt（best-effort）
- **THEN** stable prefix 不包含 tasks snapshot（best-effort）
- **AND** tasks snapshot 仅出现在本轮 TurnContext（volatile）中（best-effort）

