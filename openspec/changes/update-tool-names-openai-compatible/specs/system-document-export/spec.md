## MODIFIED Requirements

### Requirement: System MUST provide a document export tool
系统必须 (MUST) 提供一个导出工具（canonical name 为 `document_export`；并兼容 `document.export` alias），支持将 workspace 内的 Markdown 文件导出为 Office 格式，至少支持：
- `docx`
- `pptx`

#### Scenario: Export md to docx
- **GIVEN** workspace 内存在 `report.md`
- **WHEN** agent 调用 `document_export(input_path="report.md", format="docx")`
- **THEN** 系统生成 `report.docx`（或等价输出路径）
- **AND** tool output 返回输出文件路径以便验收

#### Scenario: Output path MUST remain within workspace
- **GIVEN** 用户尝试将输出写入 workspace 之外（例如 `output_path="/tmp/out.docx"`）
- **WHEN** 调用 `document_export(...)`
- **THEN** 系统拒绝执行并返回明确错误（越界）

#### Scenario: Export with template
- **GIVEN** 用户提供了一个 workspace 内的模板文件路径（例如 `template_path`）
- **WHEN** 调用 `document_export(..., template_path=...)`
- **THEN** 系统使用该模板生成输出文件（best-effort）

#### Scenario: Missing pandoc fails with actionable message
- **GIVEN** 当前环境缺少 `pandoc`
- **WHEN** 调用 `document_export(...)`
- **THEN** 系统返回明确错误
- **AND** 错误信息包含可操作的安装/启用提示（或指向 `oneagent doctor`）

