## MODIFIED Requirements

### Requirement: `read_file` 路径解析必须 workspace-aware
系统必须 (MUST) 在 workspace 启用时将相对路径解析为 workspace 内的文件；相对路径若越界（`..` 或 symlink 等导致）必须被拒绝并返回清晰错误。

系统必须 (MUST) 在工具层支持读取 workspace 之外的绝对路径文件（只读），以满足排障与参考资料读取需求；但该请求在执行时必须 (MUST) 经过 tool permissions 判定（见 `system-tool-permissions`），系统可以 (MAY) 对低信任 principal 默认拒绝以降低泄露风险。

#### Scenario: workspace 启用时相对路径越界被拒绝
- **GIVEN** workspace 根目录为 `<workspace>/`
- **WHEN** agent 调用 `read_file(filePath="../secrets.txt")`
- **THEN** 系统拒绝并返回“path is outside workspace”的清晰错误

#### Scenario: workspace 启用时策略允许读取绝对路径
- **GIVEN** workspace 根目录为 `<workspace>/`
- **AND** 当前 principal 的 policy 允许读取 `<workspace>/` 外的绝对路径
- **WHEN** agent 调用 `read_file(filePath="/tmp/notes.txt")`
- **THEN** 系统允许读取并返回内容（只读）

#### Scenario: workspace 启用时策略拒绝读取绝对路径
- **GIVEN** workspace 根目录为 `<workspace>/`
- **AND** 当前 principal 的 policy 拒绝读取 `<workspace>/` 外的绝对路径
- **WHEN** agent 调用 `read_file(filePath="/tmp/notes.txt")`
- **THEN** 系统拒绝并返回明确错误（包含拒绝原因与 principal_id）

