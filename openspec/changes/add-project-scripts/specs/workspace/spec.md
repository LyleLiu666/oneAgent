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

