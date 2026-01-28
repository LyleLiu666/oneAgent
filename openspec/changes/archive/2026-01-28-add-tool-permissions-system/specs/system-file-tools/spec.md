## ADDED Requirements

### Requirement: File tools MUST enforce policy constraints
系统必须 (MUST) 在文件写/改/删类工具中执行权限策略约束（例如 `file_scope`、`read_outside_workspace` 等）。

#### Scenario: file_scope blocks writes outside allowed glob
- **GIVEN** 当前 policy 的 `file_scope=["backend/**"]`
- **WHEN** 工具尝试写入 `frontend/App.vue`
- **THEN** 系统拒绝并返回“file_scope violation”

#### Scenario: read_outside_workspace denied by policy
- **GIVEN** policy 设置 `read_outside_workspace=deny`
- **WHEN** 工具尝试读取 workspace 外绝对路径文件
- **THEN** 系统拒绝并返回明确错误

