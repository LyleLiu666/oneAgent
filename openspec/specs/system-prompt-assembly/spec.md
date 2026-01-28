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
