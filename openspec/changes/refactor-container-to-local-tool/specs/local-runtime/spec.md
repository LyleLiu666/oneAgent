# 规范: 本地工具运行时 (Local Tool Runtime)

## ADDED Requirements

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

#### Scenario: server profile 废弃
- **WHEN** 用户执行 `oneagent serve --profile server`
- **THEN** 系统输出废弃提示
- **THEN** 系统按 `local` profile 行为启动（或直接拒绝启动并给出明确错误）

### Requirement: 规范化本地 Home/Data 目录
系统必须 (MUST) 采用一个统一的 “ONEAGENT_HOME” 目录来承载本地配置、数据、日志与工具沙箱目录，并允许通过环境变量或 CLI 参数覆盖。

#### Scenario: 显式指定 ONEAGENT_HOME
- **WHEN** 用户设置环境变量 `ONEAGENT_HOME=/tmp/oneagent-home` 并启动 `oneagent serve`
- **THEN** 系统使用该目录作为 home，并在首次启动时创建必要的子目录（config/data/sandbox/logs/tmp）

### Requirement: 不依赖 Docker 即可运行（本地工具路径）
系统必须 (MUST) 支持在不安装 Docker 的情况下完成启动与基础功能使用（在本地 profile 下）。

#### Scenario: 不安装 Docker 的用户仍可启动
- **WHEN** 用户机器未安装 Docker / Docker Compose
- **THEN** 用户仍可通过 `oneagent serve --profile local` 启动并访问 UI

### Requirement: 提供 doctor 自检
系统必须 (MUST) 提供 `oneagent doctor` 自检能力，输出运行时关键诊断信息，并能识别关键依赖缺失。

#### Scenario: 诊断信息可用于排障
- **WHEN** 用户执行 `oneagent doctor`
- **THEN** 输出包含：版本、profile、AUTH_MODE、监听地址与端口、ONEAGENT_HOME、数据库模式（sqlite/postgres/none）、关键二进制是否可用（git/rg/jq/bash）
