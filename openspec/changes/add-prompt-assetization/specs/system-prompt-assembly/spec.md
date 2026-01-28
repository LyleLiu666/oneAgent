## ADDED Requirements

### Requirement: Prompts MUST be assembled from modular assets
系统必须 (MUST) 将稳定提示词（Stable Prefix）构建为可复用的模块化资产，并允许按启用工具集与 provider 选择组装对应的 prompt。

#### Scenario: tool manuals included based on enabled tools
- **GIVEN** 本轮启用了 `bash` 与 `write_file` 工具
- **WHEN** 系统构建 stable prefix
- **THEN** stable prefix 包含 bash 与 write_file 的 tool manual 模块
- **AND** 未启用工具的 manual 不得被注入

### Requirement: Prompt key constraints MUST be testable
系统必须 (MUST) 提供自动化测试，验证 assembled prompt 中存在关键安全/稳定性约束（例如禁止输出 CDATA、禁止 heredoc 写文件等）。

#### Scenario: CI catches missing constraints
- **GIVEN** 某次修改移除了 “不要输出 CDATA” 的约束文本
- **WHEN** 运行 prompt unit tests
- **THEN** 测试失败并指出缺失的约束

