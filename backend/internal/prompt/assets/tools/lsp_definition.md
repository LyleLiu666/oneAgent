## lsp.definition 工具使用说明（简版）
- 语义导航：查定义位置（只读）；需要 workspace 已启用。
- 入参：`{ file_path, line, character }`（全部 1-based）；`file_path` 必须在 workspace 内。
- 输出：`locations[]`；用 `read_file` 打开目标位置继续分析。
