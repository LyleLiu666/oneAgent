## edit_v2 工具使用说明（简版）
- 确定性编辑（无 silent success）：必填 `filePath` + `oldString` + `newString`。
- 可选：`occurrence`（1-based）、`before_anchor/after_anchor`（消歧）、`expected_replacements`、`preconditions`（OCC）。
- 适用：匹配歧义、希望拿到 `diff_preview` 证据、或需要严格“只改这一处”。
- `newString` 可以为空（删除）；`expected_replacements=0` 表示不检查替换次数。
