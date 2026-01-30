## MODIFIED Requirements

### Requirement: Untrusted content MUST be bounded and labeled
系统必须 (MUST) 对来自外部/非可信来源的内容进行“边界包装”，并在 prompt 中明确标注其来源与可信度，降低 prompt injection 风险。

外部/非可信内容至少包括：
- Web 搜索/HTTP 抓取结果
- 剪贴板/邮件/IM 等粘贴内容
- workspace 外文件的内容（当策略允许读取时）
- 任何由工具返回、且不保证“仅为数据”的文本

当非可信内容来自 **tool output** 时，系统必须 (MUST) 额外提供一个稳定的、机器可解析的 **JSON envelope**（边界内 payload 为合法 JSON），至少包含（best-effort）：
- `_untrusted: true`
- `source.kind: "tool"`
- `source.tool: <tool name>`
- `source.tool_call_id: <id>`（如可得）
- `ok: true|false`
- `output`（当 `ok=true`；string/object）
- `error`（当 `ok=false`；包含 `error_code`、`retryable`、`message`、`hint`；best-effort）

#### Scenario: Tool output is wrapped with boundary markers AND a JSON envelope
- **GIVEN** 系统需要将某个工具的输出注入到模型上下文
- **WHEN** 该 tool output 被传入模型
- **THEN** tool output 仍被包含在明确的 begin/end 边界标记中（例如 `BEGIN_UNTRUSTED_CONTENT`/`END_UNTRUSTED_CONTENT`）
- **AND** 边界内 payload 是合法 JSON 且包含 `source.tool` 与 `ok` 字段（best-effort）
