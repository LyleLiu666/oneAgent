## ADDED Requirements

### Requirement: The system MUST define tool protocol boundaries and selection
系统必须 (MUST) 明确并固化两套工具协议（JSON 原生 tool calling / XML `<tool_data>`）的能力边界，并提供可预测的协议选择策略（best-effort）：
- 当 provider 支持原生 tools 时，系统默认使用 JSON tool calling
- 当 provider 不支持原生 tools 但 tools 被请求时，系统自动 fallback 到 XML（或返回明确错误；best-effort）

#### Scenario: Provider without tools triggers XML fallback
- **GIVEN** 用户请求启用 tools 且当前 provider 不支持原生 tools
- **WHEN** 系统准备进入工具 loop
- **THEN** 系统使用 XML tool protocol 继续推进（best-effort）
- **OR** 返回清晰错误，提示切换 provider/关闭 tools（best-effort）

### Requirement: XML protocol MUST only expose supported tools
当 tool protocol 为 XML 时，系统必须 (MUST) 只向模型暴露 XML engine 已实现参数构建与执行的工具；任何未实现的工具不得被挂载或不得被接受执行（fail-closed）。

#### Scenario: Unsupported tools are not available under XML protocol
- **GIVEN** 当前 tool protocol 为 XML
- **WHEN** 系统构建本轮可用 tools 列表
- **THEN** `lsp.*` / `document.export` 等未被 XML engine 支持的工具不会被挂载（best-effort）

### Requirement: XML protocol MUST detect truncated tool_data and request retry
当模型输出中出现 `<tool_data` 但未闭合 `</tool_data>`（截断/格式破坏）时，系统必须 (MUST) 将该 step 视为无效工具调用，并触发一次更强约束的重试指令（best-effort）。

#### Scenario: Missing closing tag triggers a stronger retry instruction
- **GIVEN** 模型输出包含 `<tool_data` 但缺失 `</tool_data>`
- **WHEN** XML tool parser 尝试解析
- **THEN** 系统判定为截断/无效结构并触发 retry（best-effort）
