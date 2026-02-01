## ADDED Requirements

### Requirement: Chat tool loop max steps MUST be configurable (JSON + XML)
系统必须 (MUST) 为 chat 工具 loop 提供可配置的最大步数（max steps），并同时适用于：
- JSON 原生 tool calling loop
- XML `<tool_data>` fallback loop

当达到最大步数时，系统必须 (MUST) 以明确错误终止本轮（不得静默卡住），并在错误信息中给出可执行的下一步建议（例如：拆分任务/减少工具调用/提高 max steps 配置）。

#### Scenario: JSON tool loop stops after configured max steps
- **GIVEN** 环境变量将 chat 工具 loop 的最大步数设置为 3
- **WHEN** 模型在每个 step 都返回至少一个 tool call（导致 loop 继续）
- **THEN** 系统在第 3 个 step 后终止并返回明确错误（best-effort）

#### Scenario: XML tool loop stops after configured max steps
- **GIVEN** 环境变量将 chat 工具 loop 的最大步数设置为 3
- **WHEN** XML 模式下模型在每个 step 都输出 `<tool_data>...</tool_data>`（导致 loop 继续）
- **THEN** 系统在第 3 个 step 后终止并返回明确错误（best-effort）

