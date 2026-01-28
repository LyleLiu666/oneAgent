## ADDED Requirements

### Requirement: Untrusted content MUST be bounded and labeled
系统必须 (MUST) 对来自外部/非可信来源的内容进行“边界包装”，并在 prompt 中明确标注其来源与可信度，降低 prompt injection 风险。

外部/非可信内容至少包括：
- Web 搜索/HTTP 抓取结果
- 剪贴板/邮件/IM 等粘贴内容
- workspace 外文件的内容（当策略允许读取时）
- 任何由工具返回、且不保证“仅为数据”的文本

#### Scenario: Tool output is wrapped with boundary markers
- **GIVEN** 系统将某段外部内容注入到对话上下文中
- **WHEN** 该内容被传入模型
- **THEN** 内容被包含在明确的 begin/end 边界标记中（例如 `BEGIN_UNTRUSTED_CONTENT`/`END_UNTRUSTED_CONTENT`）
- **AND** 同时标注来源（tool id、URL/路径等）

### Requirement: System MUST detect suspicious prompt-injection patterns and log them
系统必须 (MUST) 在将外部内容送入模型前，对“常见注入/越权/外传”模式进行 best-effort 检测（例如正则匹配），并将命中信息写入 trace/receipt（或等价审计）。

#### Scenario: Injection-like text triggers a security alert event
- **GIVEN** 外部内容包含“ignore previous instructions / reveal system prompt”等高风险模式
- **WHEN** 系统准备将该内容注入到模型上下文
- **THEN** 系统记录一条 security alert（包含命中规则/来源/摘要）
- **AND** 系统在 UI 中对该内容显示“非可信/可疑”提示（best-effort）
