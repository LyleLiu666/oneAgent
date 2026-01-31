## ADDED Requirements
### Requirement: System MUST provide a non-Docker hard-boundary sandbox for command tools (native)
系统必须 (MUST) 提供一个不依赖 Docker 的“硬边界”命令执行后端，用于 `bash`/`run_command`（例如 `sandbox_mode=native`），以便在未安装 Docker 的环境中仍可安全启用写能力命令。

该 native sandbox 必须 (MUST) 以当前会话的 `workspaceRoot` 作为默认写入边界：命令执行产生的写/删/改操作不得逃逸出 workspaceRoot（best-effort）。

#### Scenario: No Docker but native sandbox still allows in-workspace delete
- **GIVEN** 用户机器未安装 Docker
- **AND** policy 为 `bash` 指定 `sandbox_mode=native` 且 `command_profile=coding`
- **AND** workspaceRoot 内存在目录 `tmp/`
- **WHEN** LLM 通过 `bash` 执行 `rm -rf tmp/`
- **THEN** 命令执行成功并删除 `tmp/`（真删除）

#### Scenario: Native sandbox prevents out-of-workspace deletes
- **GIVEN** policy 为 `bash` 指定 `sandbox_mode=native` 且 `command_profile=coding`
- **WHEN** LLM 通过 `bash` 尝试删除 workspaceRoot 之外路径
- **THEN** 系统拒绝或由 sandbox 阻止该操作并返回可解释错误

## MODIFIED Requirements
### Requirement: Write-capable command profiles MUST require a hard-boundary sandbox_mode
系统必须 (MUST) 约束命令类工具的“可写能力”只能在硬边界执行后端中启用：
- 当 policy 将 `command_profile` 配置为 `dev/full`（或等价的“允许写入”语义）时，policy 必须同时要求 `sandbox_mode!=none`（例如 `docker` 或 `native`）。
- 若运行环境无法满足所需 sandbox（未安装/不可用），系统必须 (MUST) 拒绝执行并返回可操作错误（不得 silent fallback 为宿主执行）。

#### Scenario: dev/full profile requires hard-boundary sandbox
- **GIVEN** policy 为 `bash` 指定 `command_profile=dev` 且 `sandbox_mode=native`（或 `docker`）
- **WHEN** LLM 调用 `bash`
- **THEN** 系统在硬边界 sandbox 中执行该命令并标注 sandbox_mode

#### Scenario: Cannot fall back to host when sandbox is required
- **GIVEN** policy 要求 `sandbox_mode=native` 但 native sandbox 不可用
- **WHEN** LLM 调用 `bash`
- **THEN** 系统拒绝执行并返回可操作错误
- **AND** 系统不得在宿主机直接执行该命令作为 fallback

