## rg 工具使用说明（简版）
- 本地全文搜索（优先 rg；缺失则降级 grep/内置 Go 搜索）。
- 入参：`{ pattern(必填), path?, max_results?, fixed_strings? }`；`max_results` 默认 50，最大 200。
- 输出：`matches[]`（`path/line_number/lines/submatches`）；可能 `truncated`，必要时缩小 `path` 或更具体的 `pattern`。
