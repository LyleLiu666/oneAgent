## ADDED Requirements

### Requirement: No hard-boundary backend → command tools MUST degrade to read-only
系统必须 (MUST) 将脚本/命令执行视为高风险面，并确保其不可绕过 tool permissions 的写入边界。

当运行环境缺少“硬边界执行后端”（例如 `sandbox_mode=none`）时，系统必须 (MUST) 将命令类工具（`bash`/`run_command`）限制为只读能力（例如 `command_profile=readonly` 或等价语义）：
- 禁止任何被判定为“可能产生持久副作用”的命令执行（包括但不限于文件写/删/改、安装软件、启动守护进程）
- 禁止解释器/脚本入口（例如 `python`/`node`/`sh -c` 等）以避免间接绕过

#### Scenario: Mutating command is rejected when sandbox is unavailable
- **GIVEN** 当前执行环境 `sandbox_mode=none`
- **WHEN** LLM 尝试通过 `bash` 执行一个会写文件的命令
- **THEN** 系统拒绝执行并返回可解释错误（指出“无硬边界后端，仅允许只读命令”）

### Requirement: Write-capable command profiles MUST require a hard-boundary sandbox_mode
系统必须 (MUST) 约束命令类工具的“可写能力”只能在硬边界执行后端中启用：
- 当 policy 将 `command_profile` 配置为 `dev/full`（或等价的“允许写入”语义）时，policy 必须同时要求 `sandbox_mode!=none`（例如 `docker`）。
- 若运行环境无法满足所需 sandbox（未安装/不可用），系统必须 (MUST) 拒绝执行并返回可操作错误（不得 silent fallback 为宿主执行）。

#### Scenario: dev/full profile requires docker sandbox
- **GIVEN** policy 为 `bash` 指定 `command_profile=dev` 且 `sandbox_mode=docker`
- **WHEN** LLM 调用 `bash`
- **THEN** 系统在 docker sandbox 中执行该命令并标注 sandbox_mode

#### Scenario: Cannot fall back to host when sandbox is required
- **GIVEN** policy 要求 `sandbox_mode=docker` 但 Docker 不可用
- **WHEN** LLM 调用 `bash`
- **THEN** 系统拒绝执行并返回可操作错误
- **AND** 系统不得在宿主机直接执行该命令作为 fallback

### Requirement: Privilege escalation MUST be blocked for command tools
系统必须 (MUST) 默认拒绝命令类工具的提权行为（例如 `sudo`/`su`/修改权限以扩大写入面等），以满足“不能破坏操作系统”的底线。

#### Scenario: sudo is denied
- **WHEN** LLM 尝试通过 `bash` 执行 `sudo ...`
- **THEN** 系统拒绝并返回明确错误（包含拒绝原因）

