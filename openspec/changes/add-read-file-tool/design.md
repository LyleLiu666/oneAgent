## Design notes

### Tool contract
`read_file` 的核心目标是“可证明读到了什么”，因此输出必须包含：
- `file_path`（解析后的真实路径）
- `offset_lines`/`limit_lines`（本次请求）
- `start_line`/`end_line`（实际返回的行号区间）
- `content`（文本）
- `truncated`（是否因 `max_bytes` 或 EOF 以外原因截断）
- `total_lines`/`total_bytes`（best-effort；过大可省略或延迟计算）

### Limits
- 默认 `limit_lines` 建议 200~400（避免 tool output 过大）
- 默认 `max_bytes` 建议 64KiB（与 shell 输出截断一致，但可更小以更稳）

### Path rules
- workspace enabled:
  - relative -> resolve under workspace; MUST reject outside-workspace writes (已有)；read_file 的 relative 也 MUST reject 越界。
  - absolute -> MUST allow read-only（对齐 workspace spec）
- workspace disabled:
  - relative -> MUST reject（避免“当前目录/沙箱根目录”混淆）
  - absolute -> SHOULD allow (read-only)

