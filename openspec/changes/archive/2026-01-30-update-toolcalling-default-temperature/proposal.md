# Change: Default toolcalling temperature to 0

## Why
在启用工具调用（tool calling）时，模型需要稳定地产生结构化输出（tool selection + JSON args）。如果温度较高（或使用 provider 的默认温度），会显著提高：
- tool 参数缺字段/字段名错
- JSON 不合法/夹杂解释文本
- 随机选择错误工具、进入自愈循环

当前实现中（`backend/internal/handler/chat.go`）组装 `llm.ChatCompletionOptions` 时只设置了 `Trace`，没有对 tool-enabled 场景强制低温度默认值。

## What Changes
- 当 `tool_ids` 非空（且使用 `tool_protocol=json` 或等价“原生 tools”模式）时：
  - 若未显式配置温度，则默认将 `temperature` 设为 `0`（或 0.1，若 provider 不支持 0；best-effort）。
- 增加回归测试，确保对 OpenAI-compatible provider 的请求体包含预期的 `temperature` 字段。

## Impact
- Affected specs: `system-toolcalling-reliability`
- Affected code (expected): `backend/internal/handler/chat.go`, `backend/internal/llm/*`
- Tests (expected): `backend/internal/server/e2e_*_test.go`（mock provider 断言 request body）

