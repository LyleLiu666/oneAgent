# system-prompt-assembly Specification

## Purpose
TBD - created by archiving change add-prompt-assetization. Update Purpose after archive.
## Requirements
### Requirement: Prompts MUST be assembled from modular assets
系统必须 (MUST) 将稳定提示词（Stable Prefix）构建为可复用的模块化资产，并允许按启用工具集与 provider 选择组装对应的 prompt。

#### Scenario: tool manuals included based on enabled tools
- **GIVEN** 本轮启用了 `bash` 与 `write_file` 工具
- **WHEN** 系统构建 stable prefix
- **THEN** stable prefix 包含 bash 与 write_file 的 tool manual 模块
- **AND** 未启用工具的 manual 不得被注入

### Requirement: Assembler MUST separate stable prefix from volatile context
系统必须 (MUST) 将“稳定前缀（可缓存）”与“易变上下文（不应进入稳定前缀）”分离，避免把一次性内容（例如 skills/plan/observer/subagent summaries）污染到 stable prefix。

#### Scenario: volatile summaries are excluded from stable prefix
- **GIVEN** 本轮包含 skills/plan/observer 的动态摘要
- **WHEN** 系统构建 stable prefix
- **THEN** stable prefix 不包含这些动态摘要
- **AND** 这些摘要只出现在本轮 turn context（best-effort）

### Requirement: Prompt key constraints MUST be testable
系统必须 (MUST) 提供自动化测试，验证 assembled prompt 中存在关键安全/稳定性约束（例如禁止输出 CDATA、禁止 heredoc 写文件等）。

#### Scenario: CI catches missing constraints
- **GIVEN** 某次修改移除了 “不要输出 CDATA” 的约束文本
- **WHEN** 运行 prompt unit tests
- **THEN** 测试失败并指出缺失的约束

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

### Requirement: System MUST detect suspicious prompt-injection patterns and log them
系统必须 (MUST) 在将外部内容送入模型前，对“常见注入/越权/外传”模式进行 best-effort 检测（例如正则匹配），并将命中信息写入 trace/receipt（或等价审计）。

#### Scenario: Injection-like text triggers a security alert event
- **GIVEN** 外部内容包含“ignore previous instructions / reveal system prompt”等高风险模式
- **WHEN** 系统准备将该内容注入到模型上下文
- **THEN** 系统记录一条 security alert（包含命中规则/来源/摘要）
- **AND** 系统在 UI 中对该内容显示“非可信/可疑”提示（best-effort）

### Requirement: Core built-in tools MUST have tool manuals
系统必须 (MUST) 为核心内置工具提供 tool manuals（prompt assets），并确保在工具启用时会被注入 stable prefix。tool manuals 的文件名必须 (MUST) 与 assembler 的 `sanitizeToolName` 规则一致（例如 `lsp.definition` → `lsp_definition.md`）。

至少应覆盖（best-effort）：
- `read_file`, `write_file`
- `edit`, `edit_v2`, `multiedit`
- `rg`, `glob`, `ls`, `search`
- `run_command`, `plan`, `subagent`
- `skill.read`（manual 文件名应为 `skill_read.md`）
- `document.export`（manual 文件名应为 `document_export.md`）
- `lsp.*`（manual 文件名应为 `lsp_definition.md` / `lsp_references.md` / `lsp_rename_preview.md`，best-effort）

#### Scenario: Assembler injects manual for a core tool
- **GIVEN** 本轮启用了 `read_file` 与 `rg`
- **WHEN** 系统构建 stable prefix
- **THEN** stable prefix 包含 `read_file` 与 `rg` 的 tool manuals（best-effort）

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

