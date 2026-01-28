## ADDED Requirements

### Requirement: Command tools MUST support sandbox_mode constraints
系统必须 (MUST) 在 tool permissions policy 的 constraints 中支持对命令类工具（`bash`/`run_command`）指定 `sandbox_mode`（例如 `none`/`docker`），并在执行时强制遵守。

#### Scenario: Policy requires docker sandbox
- **GIVEN** policy 为 `bash` 指定 `sandbox_mode=docker`
- **WHEN** LLM 尝试调用 `bash`
- **THEN** 系统在 Docker sandbox 中执行命令（workspace 挂载、默认无网络）
- **AND** tool output 标注该次执行的 sandbox_mode

#### Scenario: Sandbox unavailable is actionable failure
- **GIVEN** policy 要求 `sandbox_mode=docker` 但运行环境不可用（Docker 未安装或不可用）
- **WHEN** LLM 调用 `bash`
- **THEN** 系统拒绝执行并返回可操作错误（包含如何安装/开启或如何降级策略）
