# system-toolcalling-reliability Spec Delta

## ADDED Requirements

### Requirement: Tool errors MUST be fed back into the agent loop for self-heal (best-effort)
系统必须 (MUST) 将“工具调用错误”视为 agent loop 的一等输入：当工具执行失败（包含参数不合法、权限拒绝、provider 400 等）时，系统必须 (MUST) 将失败结果写回 message list（append-only）并继续让 LLM 决策下一步（best-effort），而不是直接把工程错误暴露给用户并卡住。

约束：
- 自愈重试必须受 max steps / retry budget 约束（best-effort）
- 达到上限后，系统必须 (MUST) 才将最终失败以“用户可行动”的方式呈现（包含下一步建议 + 证据指针）

#### Scenario: Invalid tool arguments triggers self-heal retry instead of failing the task
- **GIVEN** 当前启用了工具调用（JSON 或 XML 协议均可，best-effort）
- **WHEN** 模型输出的 tool arguments 不是合法 JSON，导致 provider 返回 400（best-effort）
- **THEN** 系统将该错误作为 tool_result 写回（append-only）（best-effort）
- **AND** 系统继续下一轮 LLM 调用，让模型修复参数并重试或选择替代工具（best-effort）

### Requirement: Tool protocol selection MUST be configurable per agent (best-effort)
系统必须 (MUST) 支持按 agent 配置 tool protocol（best-effort），以避免“为兼容某个角色而全局改默认协议”：
- Worker Chat / Worker Task 可使用默认协议策略（best-effort）
- Secretary 可以配置为更稳健的协议/提示（best-effort），但不得破坏系统的稳定前缀缓存原则（见 `system-llm-prompt-caching`）

#### Scenario: Secretary forces XML tool protocol while Worker uses native tools
- **GIVEN** provider 支持原生 tools
- **AND** Secretary agent 配置 `tool_protocol=xml`（best-effort）
- **WHEN** Secretary 进入工具 loop
- **THEN** 系统使用 XML tool protocol 执行（best-effort）
- **AND** Worker Chat 仍可继续使用原生 JSON tools（best-effort）
