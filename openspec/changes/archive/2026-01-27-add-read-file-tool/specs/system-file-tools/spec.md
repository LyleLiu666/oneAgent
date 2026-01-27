## ADDED Requirements
### Requirement: 系统必须提供 `read_file` 工具（分页读取 + 输出限流）
系统必须 (MUST) 提供一个 `read_file` 工具，使 agent 可在不依赖 shell 输出的前提下读取文件内容，并支持：
- 按行分页：`offset_lines` + `limit_lines`
- 按字节上限：`max_bytes`

工具输出必须 (MUST) 返回可解释的读取结果，至少包含：
- `file_path`（解析后的路径）
- `content`
- `start_line` / `end_line`
- `truncated`（是否因限流截断）

#### Scenario: 读取小文件返回完整内容
- **GIVEN** workspace 内存在一个小文件 `a.txt`
- **WHEN** agent 调用 `read_file(filePath="a.txt", offset_lines=0, limit_lines=200, max_bytes=65536)`
- **THEN** 系统返回 `content` 包含完整文件
- **THEN** `truncated=false`

#### Scenario: 读取大文件可分页拉取
- **GIVEN** workspace 内存在一个大文件 `big.log`（行数远大于 200）
- **WHEN** agent 调用 `read_file(filePath="big.log", offset_lines=0, limit_lines=200)`
- **THEN** 系统返回 `start_line=1` 且 `end_line=200`（或等价的 1-based 行号）
- **WHEN** agent 再次调用 `read_file(filePath="big.log", offset_lines=200, limit_lines=200)`
- **THEN** 系统返回下一段内容（行号区间不重叠）

### Requirement: `read_file` 路径解析必须 workspace-aware
系统必须 (MUST) 在 workspace 启用时将相对路径解析为 workspace 内的文件；相对路径若越界（`..` 或 symlink 等导致）必须被拒绝并返回清晰错误。

系统必须 (MUST) 允许读取 workspace 之外的绝对路径文件（只读），以满足排障与参考资料读取需求。

#### Scenario: workspace 启用时相对路径越界被拒绝
- **GIVEN** workspace 根目录为 `<workspace>/`
- **WHEN** agent 调用 `read_file(filePath="../secrets.txt")`
- **THEN** 系统拒绝并返回“path is outside workspace”的清晰错误

#### Scenario: workspace 启用时允许读取绝对路径
- **GIVEN** workspace 根目录为 `<workspace>/`
- **WHEN** agent 调用 `read_file(filePath="/tmp/notes.txt")`
- **THEN** 系统允许读取并返回内容（只读）

