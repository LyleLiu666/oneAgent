## MODIFIED Requirements

### Requirement: 默认仅允许修改 workspace 内的文件
系统必须 (MUST) 默认仅允许对 workspace 根目录内的文件进行写/改/删操作（包括但不限于 `write_file` / `edit` / 删除文件等）。

系统必须 (MUST) 支持读取 workspace 之外的文件，并且在工具层支持读取任意绝对路径（只读）；但该能力在执行时必须 (MUST) 受 tool permissions 策略控制（见 `system-tool-permissions`），系统可以 (MAY) 对低信任 principal 默认拒绝读取 workspace 外文件以降低泄露风险。

#### Scenario: 写入 workspace 外文件被拒绝
- **GIVEN** 会话已启用 workspace，根目录为 `<workspace>/`
- **WHEN** 工具尝试写入或编辑 `<workspace>/` 之外的路径
- **THEN** 系统拒绝该操作并返回清晰错误（例如 “path is outside workspace”）

#### Scenario: 策略允许读取任意绝对路径时被允许
- **GIVEN** 会话已启用 workspace，根目录为 `<workspace>/`
- **AND** 当前 principal 的 policy 允许读取 `<workspace>/` 外绝对路径
- **WHEN** 工具尝试读取一个 `<workspace>/` 之外的绝对路径文件（例如 `/path/to/file`）
- **THEN** 系统允许该读取并返回文件内容（只读）

#### Scenario: 策略拒绝读取任意绝对路径时被拒绝
- **GIVEN** 会话已启用 workspace，根目录为 `<workspace>/`
- **AND** 当前 principal 的 policy 拒绝读取 `<workspace>/` 外绝对路径
- **WHEN** 工具尝试读取一个 `<workspace>/` 之外的绝对路径文件（例如 `/path/to/file`）
- **THEN** 系统拒绝该读取并返回清晰错误（例如 “read outside workspace is denied by policy”）

