## ADDED Requirements
### Requirement: 系统必须提供 `trash_file` 工具（软删除 / move-to-trash）
系统必须 (MUST) 提供一个 `trash_file` 工具，用于对 workspace 内的文件/目录执行软删除：从原路径移除，并移动到 workspace 内的系统回收站目录（例如 `<workspace>/.oneagent/trash/`）。

`trash_file` 必须 (MUST)：
- 仅允许操作 workspace 内路径（拒绝 `..` 与 symlink 逃逸导致的越界）
- 在工具输出中返回 `trash_id`、`original_path` 与 `trashed_path`（解析后的真实路径），便于审计与人工恢复
- 在执行阶段强制执行 tool permissions 中的 `file_scope` 等约束，至少对 `filePath` 进行限制

#### Scenario: trash_file 将文件移动到回收站并从原位置消失
- **GIVEN** workspace 内存在文件 `tmp/a.txt`
- **WHEN** agent 调用 `trash_file(filePath="tmp/a.txt")`
- **THEN** `tmp/a.txt` 在原位置不再存在
- **AND** 系统返回 `trashed_path` 指向 `<workspace>/.oneagent/trash/...` 下的真实路径

#### Scenario: workspace 启用时越界路径被拒绝
- **GIVEN** workspace 根目录为 `<workspace>/`
- **WHEN** agent 调用 `trash_file(filePath="../secrets.txt")`
- **THEN** 系统拒绝并返回“path is outside workspace”的清晰错误

### Requirement: 系统必须自动清理回收站条目（7 天保留期）
系统必须 (MUST) 对回收站条目实施 7 天保留期：条目创建超过 7 天后必须被永久删除（best-effort），以防止回收站无限增长。

系统必须 (MUST) 自动触发清理（best-effort），无需用户手动调用专用清理工具。

#### Scenario: 超过 7 天的回收站条目在清理后被删除
- **GIVEN** 回收站中存在一个创建时间超过 7 天的条目
- **WHEN** 系统执行一次回收站清理
- **THEN** 该条目被永久删除（payload 与 metadata 不再存在）

