## lsp_references 工具使用说明（简版）
- 语义导航：查引用位置（只读）；需要 workspace 已启用。
- 入参：`{ file_path, line, character }`（1-based），可选 `include_declaration`。
- 输出：`references[]`（可能截断，按 `hint` 缩小范围/改用 `rg` 辅助）。
- 兼容旧名（alias）：`lsp.references`。
