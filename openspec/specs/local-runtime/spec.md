# local-runtime Specification

## Purpose
TBD - created by archiving change refactor-container-to-local-tool. Update Purpose after archive.
## Requirements
### Requirement: 提供统一的本地 CLI 入口
系统必须 (MUST) 提供一个本地可执行入口（例如 `oneagent`），用于启动服务、进行自检与管理本地数据目录。

#### Scenario: 查看帮助与版本
- **WHEN** 用户执行 `oneagent --help` 或 `oneagent help`
- **THEN** 系统输出可用子命令与参数说明
- **WHEN** 用户执行 `oneagent --version`
- **THEN** 系统输出版本号与构建信息（至少包含 git commit 或 build date）

### Requirement: 支持运行画像（profile）并提供安全默认值
系统必须 (MUST) 支持至少 `local` 与 `dev` 两种运行画像，并在 `local` 模式下默认以局域网（LAN）可访问方式启动。

系统必须 (MUST) 默认启用认证（见 `auth-mode`），并在登录页面明确提示“仅建议在可信局域网内使用；在公网暴露有非常大风险”。

系统不应 (SHOULD NOT) 依赖“按客户端 IP 识别公网并阻止”的方式作为默认安全策略（IPv6 与反向代理场景下该策略不可靠）。

#### Scenario: local 模式默认监听局域网
- **WHEN** 用户执行 `oneagent serve --profile local` 且未显式指定 bind 地址
- **THEN** 服务默认监听 `0.0.0.0`（或等价的“对局域网可访问”的监听地址）

#### Scenario: local 模式支持 IPv6 bind
- **WHEN** 用户执行 `oneagent serve --profile local --bind ::`（或等价 IPv6 监听地址）
- **THEN** 服务可以正常启动并接受来自 IPv6 的请求（例如浏览器可访问 UI）

### Requirement: System MUST support a loopback-only secure bind mode
系统必须 (MUST) 支持将服务仅绑定在 loopback 地址（例如 `127.0.0.1` 或 `::1`），用于“同机访问最小暴露”模式。

#### Scenario: Serve binds loopback successfully
- **WHEN** 用户执行 `oneagent serve --bind 127.0.0.1`
- **THEN** 服务启动成功且 UI 可通过 `http://localhost:<port>` 访问

#### Scenario: Non-loopback bind shows strong warning
- **WHEN** 用户执行 `oneagent serve --bind 0.0.0.0`（或 `--bind ::`）
- **THEN** 系统在启动日志与 UI 中明确提示“局域网/公网暴露风险很大”
- **AND** 给出安全替代方案（loopback/Tailscale）

### Requirement: Doctor MUST report network exposure posture
系统必须 (MUST) 在 `oneagent doctor`（或等价诊断）中报告当前 bind 地址是否为 loopback，并在非 loopback 时给出可操作的安全建议（例如使用 loopback + Tailscale）。

#### Scenario: doctor shows actionable suggestion when exposed
- **GIVEN** 当前配置 bind 为非 loopback
- **WHEN** 用户执行 `oneagent doctor`
- **THEN** doctor 输出包含风险提示与替代建议（可操作命令或指引）

### Requirement: 支持反向代理场景（不依赖 IP 阻断）
系统必须 (MUST) 支持在反向代理（Nginx/Caddy/Traefik 等）之后运行的场景，且不得依赖“根据 RemoteIP 判定公网/内网并阻断”的逻辑作为默认安全边界（反代下 RemoteIP 往往是代理地址）。

#### Scenario: 反代后仍可正常访问
- **GIVEN** oneAgent 服务运行在反向代理之后
- **WHEN** 客户端通过代理访问 UI 并携带 `Authorization: Bearer <token>` 调用受保护 API
- **THEN** 系统正常鉴权与响应（不因反代场景而错误拒绝）

#### Scenario: server profile 废弃
- **WHEN** 用户执行 `oneagent serve --profile server`
- **THEN** 系统输出废弃提示
- **THEN** 系统按 `local` profile 行为启动（或直接拒绝启动并给出明确错误）

### Requirement: 规范化本地 Home/Data 目录
系统必须 (MUST) 采用一个统一的 `ONEAGENT_HOME` 目录作为 oneAgent 的**内部状态目录根**（配置/数据/日志），并允许通过环境变量或 CLI 参数覆盖。

系统必须 (MUST) 将 oneAgent 的可变状态统一存放在 `ONEAGENT_HOME/.oneagent/` 下（例如 `config/ data/ logs/ tmp/`），避免把内部文件散落到 workspace 根目录。

#### Scenario: 显式指定 ONEAGENT_HOME
- **WHEN** 用户设置环境变量 `ONEAGENT_HOME=/tmp/oneagent-home` 并启动 `oneagent serve`
- **THEN** 系统使用该目录作为内部状态目录根
- **THEN** 系统在首次启动时创建必要的子目录（`ONEAGENT_HOME/.oneagent/config`、`ONEAGENT_HOME/.oneagent/data`、`ONEAGENT_HOME/.oneagent/logs`、`ONEAGENT_HOME/.oneagent/tmp`）

#### Scenario: 未指定时使用默认 home
- **GIVEN** 用户未设置 `ONEAGENT_HOME` 且未通过 CLI 指定 `--home`
- **WHEN** 用户启动 `oneagent serve`
- **THEN** 系统默认使用 `~/.oneagent_default` 作为 `ONEAGENT_HOME`

### Requirement: 不依赖 Docker 即可运行（本地工具路径）
系统必须 (MUST) 支持在不安装 Docker 的情况下完成启动与基础功能使用（在本地 profile 下）。

#### Scenario: 不安装 Docker 的用户仍可启动
- **WHEN** 用户机器未安装 Docker / Docker Compose
- **THEN** 用户仍可通过 `oneagent serve --profile local` 启动并访问 UI

### Requirement: 提供 doctor 自检
系统必须 (MUST) 提供 `oneagent doctor` 自检能力，输出运行时关键诊断信息，并能识别关键依赖缺失。

#### Scenario: 诊断信息可用于排障
- **WHEN** 用户执行 `oneagent doctor`
- **THEN** 输出包含：版本、profile、AUTH_MODE、监听地址与端口、ONEAGENT_HOME、Settings 存储位置（例如 `ONEAGENT_HOME/.oneagent/settings.db`）、关键二进制是否可用（git/rg/jq/bash）

### Requirement: Windows 下可运行（Local Tool）
系统必须 (MUST) 支持在 Windows 下以本地工具形应用运行：`oneagent serve` 与 `oneagent doctor` 必须可用。

#### Scenario: Windows 上 serve/doctor 可用
- **GIVEN** 用户在 Windows 下安装了 oneAgent 可执行文件
- **WHEN** 用户执行 `oneagent serve`
- **THEN** 服务启动成功（UI + API）
- **WHEN** 用户执行 `oneagent doctor`
- **THEN** doctor 输出包含 Windows 平台的诊断信息与依赖可用性

### Requirement: Quick-start serve flags（`--open` / `--workspace`）
系统必须 (MUST) 为 `oneagent serve` 提供 quick-start flags，以降低“安装后第一次使用”的操作成本：
- `--open`：服务启动成功后，自动打开默认浏览器访问 UI。
- `--workspace <path>`：设置一个默认 workspace，供 UI 在首次进入/新会话时自动填充（用户仍可在会话级覆盖）。

#### Scenario: `--open` 自动打开 UI
- **WHEN** 用户执行 `oneagent serve --open`
- **THEN** 服务启动成功后，系统尝试打开默认浏览器访问 UI
- **THEN** 若打开失败，系统输出可操作的提示但服务仍保持运行

#### Scenario: `--workspace` 提供默认 workspace
- **WHEN** 用户执行 `oneagent serve --workspace /path/to/ws`
- **THEN** 服务向 UI 暴露 default workspace = `/path/to/ws`
- **THEN** UI 在未显式设置会话 workspace 的情况下，默认填充该 workspace

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

#### Scenario: bash 默认 profile 拒绝破坏性命令
- **GIVEN** bash 使用默认 command profile（例如 `dev`）
- **WHEN** 用户/LLM 通过 bash 尝试执行 `rm -rf ...`
- **THEN** 系统应拒绝执行并返回明确错误（说明 profile/allowlist 限制）

### Requirement: Task queue 默认 limits（steps/runtime）
系统必须 (MUST) 为 task queue 提供默认 limits（最大步骤数、最大运行时长），用于避免“无限运行/无限循环”导致资源失控。

系统应该 (SHOULD) 允许通过环境变量覆盖默认 limits 与上限（cap），以便不同团队按安全/成本约束治理。

#### Scenario: 创建 task 未指定 limits 时使用默认值
- **WHEN** 用户创建 task 且未提供 `limits.max_steps` 与 `limits.max_runtime_seconds`
- **THEN** 系统返回的 task 必须包含默认 limits
- **AND** runner 执行该 task 时必须使用同样的默认 limits

#### Scenario: 用户请求过大的 limits 会被 cap 限制
- **GIVEN** 系统配置了 limits cap
- **WHEN** 用户创建 task 时请求的 limits 超过 cap
- **THEN** 系统应将 limits 限制在 cap 范围内（并在返回的 task 中反映）

### Requirement: UI 内任务完成通知（无外部 webhook）
系统必须 (MUST) 在 UI 内提供任务状态更新的可见性，帮助用户在不持续盯屏的情况下快速发现“已完成/失败/可续跑”的任务。

#### Scenario: queued/running 进入终态时产生 UI 通知
- **GIVEN** 用户打开 Task Workbench 或 Chat 内 Task Panel
- **WHEN** 某 task 的最新 attempt 状态从 `queued` 或 `running` 变为终态（`succeeded` / `failed` / `timed_out` / `interrupted` / `canceled`）
- **THEN** UI 应显示一条任务更新提示（包含 task 标题或 id、workspace、状态）

#### Scenario: 首次加载不为历史任务产生通知
- **WHEN** 用户首次打开页面并加载任务列表
- **THEN** UI 仅建立“已知状态基线”，不为此前已完成的历史任务生成通知

### Requirement: Global tool disable switch MUST be respected
系统必须 (MUST) 保留 `ONEAGENT_DISABLE_TOOL_*` 作为全局 kill-switch，并在所有权限策略之前生效。

#### Scenario: Global disable overrides policy
- **GIVEN** `ONEAGENT_DISABLE_TOOL_WRITE_FILE=1`
- **WHEN** 任何 principal 请求使用 `write_file`
- **THEN** 系统拒绝该工具调用并返回“全局禁用”原因

### Requirement: Default cost/token limits MUST be configurable
系统必须 (MUST) 允许通过环境变量为 task queue 配置默认的 cost/token 预算与 cap，以便团队治理（例如在部门内控制预算）。

支持的环境变量（示例命名；与实现保持一致）：
- `ONEAGENT_TASK_DEFAULT_MAX_TOTAL_TOKENS`
- `ONEAGENT_TASK_MAX_TOTAL_TOKENS_CAP`
- `ONEAGENT_TASK_DEFAULT_MAX_COST_USD`
- `ONEAGENT_TASK_MAX_COST_USD_CAP`

#### Scenario: Default budgets applied when omitted
- **GIVEN** 系统配置了默认 `max_total_tokens` 与 `max_cost_usd`
- **WHEN** 用户创建 task 且未提供这些字段
- **THEN** 系统自动填充默认预算
