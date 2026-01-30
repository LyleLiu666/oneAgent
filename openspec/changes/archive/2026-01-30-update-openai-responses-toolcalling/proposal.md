# Change: Enable tool calling for OpenAI Responses provider

## Why
当前 `openai_response` provider 只支持文本输出（含 stream 增量），但**不支持工具调用**：`OpenAIResponsesClient` 没有实现 `ChatCompletionWithTools(...)` / `ChatCompletionStreamWithTools(...)`，导致在 JSON 原生 tools 模式下无法进入 `runToolLoop(...)`，工具链路直接不可用。

如果想对齐 Codex（大量依赖 Responses API），补齐 Responses tool calling 是关键工程能力之一。

参考资料（source-of-truth）：
- `docs/oneAgent_toolcall_advice/docs/04_OpenAI_Responses_API_补齐工具调用_对齐Codex关键.md`

## What Changes
- 让 `openai_response` provider 支持 tools loop：
  - MVP：先实现非流式 `ChatCompletionWithTools`（`POST /responses` with `stream=false`）
  - 解析 responses `output` items，抽取 `output_text` 与 `tool_call`，并组装为统一的 `ChatCompletionResult`
  - 在 tool loop 中能正确插入 tool result（基于 tool_call_id）并继续下一轮
- 流式 tools（`ChatCompletionStreamWithTools`）作为后续增强：若尚未实现，应明确禁止或降级，并返回可操作提示（best-effort）。

## Impact
- Affected specs: `llm-provider-management`
- Affected code (expected): `backend/internal/llm/openai_responses.go`, `backend/internal/handler/chat.go`
- Tests (expected): `backend/internal/llm/*_test.go`, `backend/internal/server/e2e_*_test.go`（mock `/responses`）

