# workspace Specification

## Purpose
Defines the workspace concept and safety boundaries, including tool scope alignment, onboarding and project config discovery, and optional worktree execution mode for attempts.
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

### Requirement: Workspace project config MUST be discoverable
系统必须 (MUST) 支持在 workspace 根目录下发现并读取 project config 文件：`.oneagent/project.json`。

该 config 用于描述项目级 workflow（例如 setup/test/cleanup/dev server），以提升可复现性与“开箱即用”体验。

#### Scenario: workspace 存在 project config 时可被读取
- **GIVEN** workspace 根目录存在 `.oneagent/project.json`
- **WHEN** 系统为该 workspace 创建会话或创建 task attempt
- **THEN** 系统读取并解析该文件
- **AND** 解析结果可被系统用于后续执行（例如 attempt lifecycle scripts）

#### Scenario: project config 不存在时系统正常工作
- **GIVEN** workspace 根目录不存在 `.oneagent/project.json`
- **WHEN** 用户创建会话或创建 task
- **THEN** 系统不应报错
- **AND** 系统以“无 project config”模式继续工作（best-effort）

#### Scenario: project config 无法解析时返回可操作错误
- **GIVEN** workspace 根目录存在 `.oneagent/project.json` 但内容无法解析（例如 JSON 非法/字段类型错误）
- **WHEN** 系统尝试读取该 config
- **THEN** 系统返回清晰错误（指出文件路径与原因）
- **AND** 系统不得 silent fallback（避免用户误以为脚本被执行）

### Requirement: Project config MUST support scripts and copy_files schema
系统必须 (MUST) 为 `.oneagent/project.json` 定义并校验一个明确的 schema（防止拼写错误导致 silent no-op），至少支持以下可选字段：
- `setup_script`：string
- `test_script`：string
- `cleanup_script`：string
- `dev_server_script`：string（用于未来的 preview/dev-server 场景）
- `copy_files`：string[]（相对 workspace root 的文件路径列表）

系统必须 (MUST) 将 `copy_files` 视为“显式声明需要复制进执行环境的文件清单”（例如 `.env`），并对每个条目执行路径校验，确保解析后的路径不逃逸出 workspace root。

#### Scenario: project config schema 校验通过后可被使用
- **GIVEN** workspace 根目录存在合法的 `.oneagent/project.json`（字段类型正确）
- **WHEN** 系统读取并解析该文件
- **THEN** 系统将解析结果作为该 workspace 的 project config（或等价结构）供后续流程使用

#### Scenario: copy_files 包含逃逸路径时被拒绝
- **GIVEN** `.oneagent/project.json` 的 `copy_files` 包含 `../secrets.env`（或等价可逃逸路径）
- **WHEN** 系统读取并解析该文件
- **THEN** 系统拒绝该 config 并返回可操作错误（指出字段与非法条目）

### Requirement: Tool-enabled sessions MUST preflight workspace requirements
当会话请求启用工具（例如 `tool_ids` 非空）且所启用工具依赖 workspaceRoot 作为默认作用域时，系统必须 (MUST) 在调用 LLM 前执行 workspace 前置校验：
- workspace 未设置 → fail-fast 返回可操作错误
- workspace 已设置 → 正常进入 LLM / tool loop（best-effort）

#### Scenario: Tools enabled but workspace missing fails fast
- **GIVEN** 用户发起聊天请求并启用 tools
- **AND** 当前会话未设置 workspaceRoot
- **WHEN** 系统准备调用 LLM
- **THEN** 系统直接返回“workspace 未设置”的可操作错误
- **AND** 系统不应调用 LLM（best-effort）

### Requirement: Workspace MUST support optional worktree execution mode for attempts
系统必须 (MUST) 支持为 workspace 配置一种可选的 task attempt 执行模式 `worktree`（仅在 workspace 是 git repo 时生效）。

当 `worktree` 模式启用时，系统必须 (MUST) 为每个 attempt 选择一个独立的“执行根目录”（worktree root），并把它作为文件/搜索/命令工具的默认作用域与写入边界。

当 workspace 不是 git repo 时，系统必须 (MUST) 明确返回可操作错误或按配置退化到 `workspace` 模式（不得 silent fallback）。

#### Scenario: git workspace 启用 worktree mode 后 attempt 使用独立执行根目录
- **GIVEN** workspace 是 git repo
- **AND** workspace 启用了 attempt 执行模式 `worktree`
- **WHEN** 系统创建一个新的 task attempt
- **THEN** 该 attempt 的执行根目录为一个独立 worktree 路径（不等于 workspace root）
- **AND** 文件/命令工具默认在该 worktree 路径下执行

#### Scenario: non-git workspace 启用 worktree mode 返回可操作错误
- **GIVEN** workspace 不是 git repo
- **AND** workspace 启用了 attempt 执行模式 `worktree`
- **WHEN** 系统尝试创建一个新的 task attempt
- **THEN** 系统返回清晰错误（指出“workspace 非 git repo，无法创建 worktree”）

### Requirement: Worktree mode MUST fail closed for non-git workspaces unless an explicit fallback is configured
When workspace execution mode is `worktree` and the workspace is not a git repository, the system MUST fail with an actionable error by default, and MUST NOT silently fall back to mutable in-place execution.

#### Scenario: Non-git workspace in worktree mode is rejected by default
- **GIVEN** workspace execution mode is configured as `worktree`
- **AND** the target workspace is not a git repository
- **WHEN** a new mutating attempt is created
- **THEN** the system returns an actionable error
- **AND** no mutating attempt is started in-place

### Requirement: Worktree execution root MUST remain the default write boundary for file and command tools
For attempts running in worktree mode, file tools and command tools MUST use `worktree_root` as their default scope and write boundary (subject to policy), not the original workspace root.

#### Scenario: Tool writes are scoped to worktree_root
- **GIVEN** an attempt is running in worktree mode
- **WHEN** a mutating file or command tool is executed
- **THEN** writes are constrained to `worktree_root` by default
- **AND** attempts to escape that root are rejected by policy or boundary checks

