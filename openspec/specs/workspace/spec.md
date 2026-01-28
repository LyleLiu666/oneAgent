# workspace Specification

## Purpose
TBD - created by archiving change refactor-container-to-local-tool. Update Purpose after archive.
## Requirements
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

### Requirement: 工具默认作用域对齐 workspace
系统必须 (MUST) 将“文件工具/搜索工具/命令执行工具”的默认作用域对齐到当前会话的 workspace。

#### Scenario: 搜索默认在 workspace 内进行
- **GIVEN** 会话已启用 workspace，根目录为 `<workspace>/`
- **WHEN** 用户在对话中请求搜索代码（例如使用 `rg`）
- **THEN** 搜索在 `<workspace>/` 内执行（不默认扫描 workspace 外目录）

### Requirement: 跨平台 workspace 选择（picker 可选，手动输入必备）
系统必须 (MUST) 在所有平台提供“手动输入 workspace 路径并立即校验/规范化”的能力，确保用户不会因为缺少 native folder picker 而无法启用 workspace。

系统应该 (SHOULD) 在 local-tool 场景提供 OS-native 的 workspace folder picker（例如 Windows/macOS）。

#### Scenario: Windows 上可通过 picker 选择 workspace
- **GIVEN** oneAgent 运行在 Windows 且处于 local-tool 场景（UI 与服务同机）
- **WHEN** 用户在 UI 中点击 “Browse/Choose workspace”
- **THEN** 系统打开 folder picker 并返回用户选择的目录
- **THEN** 系统对目录进行规范化并作为 workspace 保存

#### Scenario: picker 不可用时可手动输入并通过校验
- **GIVEN** 系统不支持 folder picker（例如远程 server、或平台限制）
- **WHEN** 用户在 UI 输入一个 workspace 目录路径并保存
- **THEN** 系统对该路径进行规范化与存在性校验
- **THEN** 校验通过后该路径作为 workspace 生效
- **THEN** 校验失败时返回清晰错误原因（例如路径不存在/不可访问）

### Requirement: Workspace-first onboarding（优先引导选择 workspace）
系统必须 (MUST) 在用户首次进入 UI 或新建会话时，优先引导用户选择一个本地目录作为 workspace，以便立即使用文件/搜索/命令等工具能力。

系统仍必须 (MUST) 支持用户跳过 workspace，进入“仅对话”模式（保持 `workspace` 能力的可选性）。

#### Scenario: 首次进入且未设置 workspace 时展示引导
- **GIVEN** 用户首次进入聊天页面且当前会话未设置 workspace
- **WHEN** UI 渲染空态（尚未开始对话）
- **THEN** UI 明确提示“选择 workspace 后可直接使用文件/命令工具”
- **THEN** UI 提供“一键选择文件夹”的入口（例如 Browse）
- **THEN** UI 提供“跳过（仅对话）”入口

#### Scenario: 选择 workspace 后立即可开始工作
- **GIVEN** 用户处于 onboarding 引导状态
- **WHEN** 用户选择一个有效目录作为 workspace
- **THEN** UI 将该路径保存为当前会话 workspace
- **THEN** 文件/搜索/命令工具的默认作用域变为该 workspace
- **THEN** 输入框获得焦点，用户可立即发送消息开始工作

#### Scenario: 记住最近 workspace 以减少重复操作
- **GIVEN** 用户曾在该浏览器/客户端上选择过 workspace
- **WHEN** 用户再次进入聊天页面且当前会话未设置 workspace
- **THEN** UI 默认填充最近一次 workspace 并提示用户可一键确认或修改

