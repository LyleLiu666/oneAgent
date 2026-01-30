## read_file 工具使用说明（简版）
- 用于读取文件内容；改/写文件前先用它确认上下文与行号。
- 必填：`filePath`（相对路径要求已启用 workspace；绝对路径只读允许）。
- 可选：`offset_lines`/`limit_lines`/`max_bytes`（分页 + 控输出）。
- 返回：`ok`, `file_path`, `start_line/end_line`, `content`, `truncated`（若截断则继续分页读取）。
