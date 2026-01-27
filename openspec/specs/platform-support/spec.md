# platform-support Specification

## Purpose
TBD - created by archiving change add-windows-support. Update Purpose after archive.
## Requirements
### Requirement: Windows 作为一级支持平台（Build & Run）
系统必须 (MUST) 支持在 Windows 平台构建并运行 oneAgent（至少包括 `oneagent serve` 与 `oneagent doctor`）。

#### Scenario: Windows 上可启动服务并访问 UI
- **GIVEN** 用户使用 Windows 机器
- **WHEN** 用户执行 `oneagent serve`
- **THEN** 服务可启动并监听端口
- **THEN** 用户可通过浏览器访问 UI

#### Scenario: Windows 上 doctor 可用于排障
- **GIVEN** 用户使用 Windows 机器
- **WHEN** 用户执行 `oneagent doctor`
- **THEN** 输出包含运行画像/监听信息/ONEAGENT_HOME 等诊断信息
- **THEN** 输出包含关键依赖可用性（例如 shell runner、搜索后端等）

### Requirement: 跨平台工具依赖可降级且可诊断
系统必须 (MUST) 在不同平台上提供可用的“命令执行”和“本地搜索”能力，或在能力不可用时给出清晰可操作的降级提示（不得静默失败）。

#### Scenario: 缺少外部依赖时仍可使用基本能力
- **GIVEN** Windows 环境未安装 `rg` 且不存在 `grep`
- **WHEN** 用户触发技能召回或本地搜索
- **THEN** 系统仍可通过等价降级实现返回结果（允许性能下降）
- **THEN** 系统在结果或 doctor 中提供可操作的提示（例如如何启用更快的搜索后端）

### Requirement: Windows 命令执行默认使用 Git Bash（优先 bundled）
系统必须 (MUST) 在 Windows 上默认使用 Git Bash 作为命令执行后端（至少覆盖 `bash` 工具与 `run_command` 工具的默认路径）。

系统应该 (SHOULD) 在 Windows release 已内置 Git Bash 时优先使用 bundled `bash.exe`，以确保一致的运行环境与可复现性。

#### Scenario: Windows release 使用 bundled Git Bash 执行命令
- **GIVEN** oneAgent 运行在 Windows release（内置 Git/Git Bash）
- **WHEN** oneAgent 需要执行 `bash` 或 `run_command`
- **THEN** oneAgent 使用 bundled 的 `bash.exe` 执行命令（例如 `bash.exe -lc <cmd>`）
- **THEN** `oneagent doctor` 可输出 `bash_source=bundled`

#### Scenario: bash 不可用时显式报错并给出替代路径
- **GIVEN** oneAgent 运行在 Windows 且无法找到可用的 `bash.exe`（system 与 bundled 均不可用，或被策略拦截）
- **WHEN** oneAgent 需要执行 `bash` 或 `run_command`
- **THEN** 系统必须返回明确错误（不得隐式 fallback 到其它 shell）
- **THEN** 错误信息或 `oneagent doctor` 提供可操作的解决方案（例如使用带 bundled Git Bash 的 release 或安装 Git for Windows）

### Requirement: Windows release 内置 Git 与 Git Bash（portable）
系统必须 (MUST) 在 Windows 的 release 产物中内置一套可用的 Git（portable，包含 Git Bash），用于在未安装 Git 的机器上提供一致的 `git`/`bash` 能力。

系统必须 (MUST) 在运行时能清晰区分并输出当前使用的是系统依赖还是内置依赖（例如 `doctor` 输出 `git_source=system|bundled` 与 `bash_source=system|bundled`），以便排障与审计。

#### Scenario: 系统未安装 git 时使用内置 git
- **GIVEN** Windows 机器未安装 Git（PATH 中不存在 `git.exe`）
- **WHEN** oneAgent 运行并需要使用 `git` 能力（或 doctor 检查 git）
- **THEN** oneAgent 使用 Windows release 内置的 `git.exe`（bundled）

#### Scenario: doctor 输出 git/bash 来源
- **GIVEN** oneAgent 运行在 Windows
- **WHEN** 用户执行 `oneagent doctor`
- **THEN** 输出包含 git 与 bash 的可用性与来源（system 或 bundled）

### Requirement: Windows Release 产物可分发
系统必须 (MUST) 提供 Windows 的 release 产物（例如 zip），并提供校验文件（checksums），以支持部门内分发与安装校验。

#### Scenario: Release 包含 Windows 产物与 checksums
- **WHEN** 执行 release 打包流程
- **THEN** 产物目录包含 Windows 平台的可执行文件（例如 `oneagent_..._windows_amd64.exe` 或等价命名）
- **THEN** 产物目录包含对应的 checksums 文件

#### Scenario: bundled 依赖的许可证/NOTICE 随包分发
- **GIVEN** Windows release 产物内置了 Git/Git Bash 或其它第三方可执行文件
- **WHEN** 执行 release 打包流程
- **THEN** release 包内包含对应的许可证/NOTICE 文件（满足分发合规）

