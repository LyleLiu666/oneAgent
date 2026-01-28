## ADDED Requirements

### Requirement: System MUST provide semantic code navigation tools
系统必须 (MUST) 提供语义级代码导航工具（LSP/AST），至少包括：
- `lsp.definition`
- `lsp.references`

这些工具必须 (MUST) 是只读的，并以 workspace 为边界强制路径/范围合法性。

#### Scenario: definition returns stable locations
- **GIVEN** workspace 中存在一个 Go 符号定义与引用
- **WHEN** agent 调用 `lsp.definition` 查询该符号
- **THEN** 系统返回该定义的文件路径与行列位置
- **AND** 返回结果稳定排序

### Requirement: System MUST provide `lsp.rename_preview` without writing files
系统必须 (MUST) 提供 `lsp.rename_preview`，用于生成“重命名将修改哪些位置”的改动清单。

该工具不得 (MUST NOT) 直接修改任何文件；文件改动必须由 `edit`/`edit_v2`/`write_file` 等写工具显式执行。

#### Scenario: rename_preview outputs edits list only
- **GIVEN** workspace 中存在多个对某符号的引用
- **WHEN** agent 调用 `lsp.rename_preview(new_name=...)`
- **THEN** 系统返回一个 edits 列表（包含每个文件的改动范围与新文本）
- **AND** workspace 内文件内容未被修改

