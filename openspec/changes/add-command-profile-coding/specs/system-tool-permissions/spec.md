## MODIFIED Requirements

### Requirement: Command tools MUST support profiles (readonly/dev/full)
系统必须 (MUST) 为 `bash`/`run_command` 提供可配置的命令 profile（`readonly/dev/coding/full`），默认使用最小权限集合，并优先采用 allowlist。

其中 `coding` profile 仅用于 coding agent 场景，必须 (MUST) 结合强隔离 sandbox（例如 `sandbox_mode=docker`）使用；当环境不满足时必须 fail-closed 并返回可操作错误（best-effort）。

#### Scenario: readonly profile blocks interpreters
- **GIVEN** 当前 profile 为 `readonly`
- **WHEN** 调用 `bash` 执行 `python`/`node` 等解释器
- **THEN** 系统拒绝该命令并返回可解释错误

#### Scenario: coding profile requires docker sandbox
- **GIVEN** 当前 profile 为 `coding` 且 policy 要求 `sandbox_mode=docker`
- **WHEN** 调用 `run_command` 执行 `git status`（best-effort）
- **THEN** 系统在 docker sandbox 中执行或返回可操作的环境缺失错误（best-effort）

