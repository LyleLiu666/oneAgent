## ADDED Requirements

### Requirement: OpenAI Responses provider MUST support tool calling
系统必须 (MUST) 允许在 `provider_type=openai_response` 下启用工具调用，并可进入与 `openai`（Chat Completions）一致的 tools loop（best-effort）。

该能力至少必须覆盖（MVP）：
- 非流式 tools：`ChatCompletionWithTools(...)` 通过 `POST /responses`（`stream=false`）返回 `tool_call` 与最终文本输出（best-effort）。

#### Scenario: Responses provider executes a non-stream tools loop
- **GIVEN** 当前会话选择的 provider 为 `openai_response` 且启用 tools（`tool_ids` 非空）
- **WHEN** 模型返回一个 `tool_call`（包含 `id/name/arguments`）
- **THEN** 系统执行该工具并将 tool result 关联到 `tool_call_id`
- **AND** 系统继续下一轮 responses 调用直至返回最终内容（best-effort）

### Requirement: Responses tools MVP MUST fail closed for unsupported stream+tools
在 Responses tools 尚未完整支持流式增量拼接前（best-effort），系统必须 (MUST) 对 “stream=true 且 tools 启用” 的组合返回可操作结果：
- 要么强制降级为非流式（best-effort）
- 要么返回清晰错误，指向替代方案（例如关闭 streaming 或切换 provider）

#### Scenario: Streaming with tools is rejected or downgraded
- **GIVEN** provider 为 `openai_response` 且 tools 启用
- **WHEN** 客户端请求 `stream=true`
- **THEN** 系统降级为非流式或返回可操作错误（best-effort）
