## ADDED Requirements

### Requirement: 支持 OCC 自动预条件写入（L2）
系统必须 (MUST) 支持基于文件指纹（sha256）的条件写入（OCC），并在启用 OCC 自动化时提供“read→write/edit”闭环以避免版本漂移。

#### Scenario: read 后文件被外部修改，write 自动拒绝
- **GIVEN** OCC 自动化启用且系统已通过 `read_file` 记录某文件的版本指纹
- **WHEN** 该文件在工具写入前被外部修改
- **THEN** 随后的 `write_file`/`edit` 在未显式提供 `preconditions` 时仍应自动带上 `expected_sha256` 并拒绝写入
- **AND** 错误信息应提示需要重新读取文件后再修改

#### Scenario: 连续多次 edit 不应因为 OCC 自我冲突失败
- **GIVEN** OCC 自动化启用且系统已记录某文件指纹
- **WHEN** 同一任务连续多次对该文件进行 `edit`/`write_file`
- **THEN** 写入成功后系统应更新指纹，使后续编辑不会因“预期 sha 过旧”而失败

### Requirement: 工具权限控制（禁用与破坏性命令保护）
系统必须 (MUST) 提供最小可用的工具权限控制能力，以便在本地/单机模式下限制风险。

#### Scenario: 通过环境变量禁用工具
- **WHEN** 用户设置 `ONEAGENT_DISABLE_TOOL_BASH=1`（或等价）
- **THEN** 系统不得向 LLM 暴露该工具
- **AND** 若用户/系统显式请求该工具，应返回明确错误（包含 tool id 与禁用原因）

#### Scenario: bash 默认拒绝 rm
- **GIVEN** 未设置 `ONEAGENT_BASH_ALLOW_RM=1`
- **WHEN** 用户/LLM 通过 bash 尝试执行包含 `rm` 的命令
- **THEN** 系统应拒绝执行并返回明确错误

