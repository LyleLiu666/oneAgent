## lsp.rename_preview 工具使用说明（简版）
- 语义重命名预览（只读，不写文件）；需要 workspace 已启用。
- 入参：`{ file_path, line, character, new_name }`（全部 1-based）。
- 输出：`edits[]`（预览将改动的位置）；再用 `edit/edit_v2` 按 edits 应用修改。
