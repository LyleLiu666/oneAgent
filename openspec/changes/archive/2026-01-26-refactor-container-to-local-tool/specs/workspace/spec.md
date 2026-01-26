# 规范: Workspace（Project）与文件作用域 (Workspace & File Scope)

## ADDED Requirements

### Requirement: Workspace 是可选的“项目根目录”
系统必须 (MUST) 支持一个可选的 `workspace` 概念：它是一个本地目录路径，对应 coding 场景下的 project 根目录。

当会话启用 workspace 时，系统必须 (MUST) 保存 workspace 根目录路径，并将其作为 **agent 可修改文件的最大范围**（文件工具/搜索工具/命令执行工具默认以此为作用域）。

`ONEAGENT_HOME` 仍用于承载 oneAgent 的内部状态目录（例如 `ONEAGENT_HOME/.oneagent/*`）；用户可选择将 `ONEAGENT_HOME=<workspace>` 以获得“项目私有数据”体验，但系统不应依赖自动切换来实现 workspace 功能。

当用户开启一个新会话时，系统必须 (MUST) 允许用户：
- 不启用 workspace（仅对话，不进行文件改动）
- 选择一个已存在的 workspace
- 或选择一个新的本地目录作为 workspace

#### Scenario: 新建会话可选择 workspace
- **GIVEN** 用户打开“新建会话”
- **WHEN** 用户选择某个本地目录作为 workspace
- **THEN** 该会话被标记为“启用 workspace”，并保存 workspace 根目录路径

### Requirement: 默认仅允许修改 workspace 内的文件
系统必须 (MUST) 默认仅允许对 workspace 根目录内的文件进行写/改/删操作（包括但不限于 `write_file` / `edit` / 删除文件等）。

系统必须 (MUST) 支持读取 workspace 之外的文件，并且必须允许读取任意绝对路径（例如 `/path/to/file`）。读取必须以“只读”方式进行，且系统不得 (MUST NOT) 默认提供对 workspace 外文件的写入能力。

#### Scenario: 写入 workspace 外文件被拒绝
- **GIVEN** 会话已启用 workspace，根目录为 `<workspace>/`
- **WHEN** 工具尝试写入或编辑 `<workspace>/` 之外的路径
- **THEN** 系统拒绝该操作并返回清晰错误（例如 “path is outside workspace”）

#### Scenario: 读取任意绝对路径被允许
- **GIVEN** 会话已启用 workspace，根目录为 `<workspace>/`
- **WHEN** 工具尝试读取一个 `<workspace>/` 之外的绝对路径文件（例如 `/path/to/file`）
- **THEN** 系统允许该读取并返回文件内容（只读）

### Requirement: 工具默认作用域对齐 workspace
系统必须 (MUST) 将“文件工具/搜索工具/命令执行工具”的默认作用域对齐到当前会话的 workspace。

#### Scenario: 搜索默认在 workspace 内进行
- **GIVEN** 会话已启用 workspace，根目录为 `<workspace>/`
- **WHEN** 用户在对话中请求搜索代码（例如使用 `rg`）
- **THEN** 搜索在 `<workspace>/` 内执行（不默认扫描 workspace 外目录）
