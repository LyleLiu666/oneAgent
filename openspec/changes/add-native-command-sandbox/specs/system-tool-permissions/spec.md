## MODIFIED Requirements
### Requirement: Command tools MUST support profiles (readonly/dev/full)
系统必须 (MUST) 为 `bash`/`run_command` 提供可配置的命令 profile（`readonly/dev/coding/full`），默认使用最小权限集合，并优先采用 allowlist。

其中 `coding` profile 仅用于 coding agent 场景，必须 (MUST) 结合强隔离 sandbox 使用；当环境不满足时必须 fail-closed 并返回可操作错误（best-effort）。

#### Scenario: readonly profile blocks interpreters
- **GIVEN** 当前 profile 为 `readonly`
- **WHEN** 调用 `bash` 执行 `python`/`node` 等解释器
- **THEN** 系统拒绝该命令并返回可解释错误

#### Scenario: coding profile requires hard-boundary sandbox
- **GIVEN** 当前 profile 为 `coding` 且 policy 要求 `sandbox_mode=native`（或 `docker`）
- **WHEN** 调用 `run_command` 执行 `git status`（best-effort）
- **THEN** 系统在硬边界 sandbox 中执行或返回可操作的环境缺失错误（best-effort）

### Requirement: Command tools MUST support sandbox_mode constraints
系统必须 (MUST) 在 tool permissions policy 的 constraints 中支持对命令类工具（`bash`/`run_command`）指定 `sandbox_mode`（例如 `none`/`docker`/`native`），并在执行时强制遵守。

#### Scenario: Policy requires native sandbox
- **GIVEN** policy 为 `bash` 或 `run_command` 指定 `sandbox_mode=native`
- **WHEN** LLM 尝试调用该工具执行会写/删文件的命令（例如 `rm -rf tmp/`）
- **THEN** 系统在 native sandbox 中执行命令
- **AND** tool output 标注该次执行的 sandbox_mode

#### Scenario: Native sandbox unavailable is actionable failure
- **GIVEN** policy 要求 `sandbox_mode=native` 但运行环境不可用（平台不支持/依赖缺失/初始化失败）
- **WHEN** LLM 调用该工具
- **THEN** 系统拒绝执行并返回可操作错误（包含如何启用/安装/修复或如何调整策略）

