## ADDED Requirements

### Requirement: `write_file` MUST NOT silently truncate content
系统必须 (MUST) 确保 `write_file` 不会在无错误信号的情况下写入“被截断的内容”并返回成功（silent partial write）。

当单次入参 `content` 超过系统允许的最大大小时，系统必须 (MUST)：
- 明确失败（tool result 中 `ok=false` 且返回可理解的错误信息）
- 不写入/不改动目标文件（避免产生半成品）
- 给出 best-effort 的下一步建议（例如使用 `append=true` 分段写入）

#### Scenario: Oversize content fails without modifying existing file
- **GIVEN** 文件 `a.txt` 已存在且内容为 `old`
- **WHEN** agent 调用 `write_file(filePath="a.txt", content=<oversize>)`
- **THEN** 工具返回 `ok=false`
- **AND** `a.txt` 内容仍为 `old`

