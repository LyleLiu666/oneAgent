## edit 工具使用说明（简版）
- 对已有文件做模糊替换（fuzzy patch）；整文件写入/新建用 `write_file`。
- 入参：`{ edits: [{ filePath, oldString, newString, replaceAll?, preconditions? }], replaceAll? }`
- 建议：每次只改 1-3 处；单次 `oldString/newString` 建议 ≤3000 字，避免超长导致工具调用失败。
- 匹配歧义/需要证据时优先用 `edit_v2`（支持 occurrence/anchors + diff 预览）。
- OCC：可传 `preconditions.expected_sha256` 防止文件变更导致误改。
