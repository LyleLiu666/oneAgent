# Change: Make tool outputs/errors machine-parseable (JSON envelope)

## Why
当前工具输出采用 `BEGIN_UNTRUSTED_CONTENT ... END_UNTRUSTED_CONTENT` 的文本包装形式，但 tool output / tool error 往往是“人类可读、机器难读”的自由文本，模型在自愈时难以稳定提取关键信息（例如：是哪类失败、是否可重试、下一步该换什么工具/参数）。

这会放大“卡住/不清晰”的体验：同一类错误在 tool loop 中反复出现，步数暴涨，用户很快失去耐心。

专家建议（source-of-truth）：
- `docs/oneAgent_toolcall_advice/docs/02_JSON工具调用_可靠性提升方案_可落地改动.md`（工具输出 envelope + 可行动错误）
- `docs/oneAgent_toolcall_advice/docs/08_Roadmap_分层次推进_由简单到长远.md`（L1-3 工具错误分类 + 自愈提示）

## What Changes
- 对“注入到模型上下文的工具输出/工具错误”增加一个稳定的 **JSON envelope**：
  - 仍保留 `BEGIN_UNTRUSTED_CONTENT/END_UNTRUSTED_CONTENT` 边界（安全语义不变）
  - 但边界内的 payload 必须是**合法 JSON**，包含：
    - tool 名称、tool_call_id、ok=true/false
    - 输出（string/object）或结构化 error（error_code/retryable/hint 等；best-effort）
- 错误 payload 必须尽量“可行动”（best-effort），帮助模型更快自愈：
  - policy denied：提示替代工具/做法
  - invalid arguments：提示“只返回 JSON 对象，不要解释”，并指出缺失字段
  - workspace 未设置：提示用户先设置 workspaceRoot

## Impact
- Affected specs: `system-prompt-assembly`
- Affected code (expected): `backend/internal/handler/untrusted_content.go`, `backend/internal/handler/chat.go`, `backend/internal/tool/*`
- Tests (expected): prompt unit tests + tool loop tests

