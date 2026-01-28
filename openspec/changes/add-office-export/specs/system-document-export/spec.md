## ADDED Requirements

### Requirement: System MUST provide a document export tool
系统必须 (MUST) 提供一个导出工具（例如 `document.export`），支持将 workspace 内的 Markdown 文件导出为 Office 格式，至少支持：
- `docx`
- `pptx`

#### Scenario: Export md to docx
- **GIVEN** workspace 内存在 `report.md`
- **WHEN** agent 调用 `document.export(input_path=\"report.md\", format=\"docx\")`
- **THEN** 系统生成 `report.docx`（或等价输出路径）
- **AND** tool output 返回输出文件路径以便验收

#### Scenario: Missing pandoc fails with actionable message
- **GIVEN** 当前环境缺少 `pandoc`
- **WHEN** 调用 `document.export(...)`
- **THEN** 系统返回明确错误
- **AND** 错误信息包含可操作的安装/启用提示（或指向 `oneagent doctor`）

