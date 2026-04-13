## ADDED Requirements
### Requirement: Agent Factory MUST delegate shared runtime-kernel semantics to the external agentsdk (best-effort)
系统必须 (MUST) 通过 Agent Factory（或等价统一入口）把“跨宿主共享的运行内核语义”委托给外部 `agentsdk`（best-effort），避免在 `oneAgent` 内重复维护一套同类规则。

这里的“共享运行内核语义”至少包括（best-effort）：
- prompt cache message behavior（`volatile` / `force_cacheable`）
- `prompt_cache_key` 生成规则
- tool arguments 容错模式（`best_effort` / `strict`）
- 后续复用的 JSON/XML tool loop 公共策略

`oneAgent` 宿主层仍负责：
- session / message persistence
- permissions / settings
- secretary / task queue / work ledger
- SSE / trace / llm log / UI

#### Scenario: Factory keeps host responsibilities local while reusing SDK kernel semantics
- **GIVEN** `oneAgent` 通过 Agent Factory 构建 worker 或 secretary agent（best-effort）
- **WHEN** 该 agent 需要使用 prompt cache 或 tool arguments recovery（best-effort）
- **THEN** 共享运行内核语义来自外部 `agentsdk`（best-effort）
- **AND** session 持久化、权限策略、SSE 与 trace 仍由 `oneAgent` 宿主层负责（best-effort）
