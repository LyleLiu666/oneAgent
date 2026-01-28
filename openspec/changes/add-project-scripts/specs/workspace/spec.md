## ADDED Requirements

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
