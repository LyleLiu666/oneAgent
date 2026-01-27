## MODIFIED Requirements
### Requirement: 读取任意绝对路径被允许（通过 `read_file` 工具）
当会话启用 workspace 时，系统必须 (MUST) 允许通过 `read_file` 工具读取 workspace 之外的绝对路径文件（只读），以支持排障与参考资料读取；系统不得 (MUST NOT) 因为路径不在 workspace 内而拒绝读取绝对路径。

#### Scenario: 读取任意绝对路径被允许
- **GIVEN** 会话已启用 workspace，根目录为 `<workspace>/`
- **WHEN** 工具尝试读取一个 `<workspace>/` 之外的绝对路径文件（例如 `/path/to/file`）
- **THEN** 系统允许该读取并返回文件内容（只读）

