# Change: 将 oneAgent 从“容器运行”改造为“本地工具形应用”

## 摘要 (Summary)
当前 oneAgent 的默认运行方式是通过 Docker / Docker Compose（至少包含 `backend` 与 `postgres` 容器），并依赖外部 Keycloak（或未来可选的本地 Keycloak）。我希望将项目改造成一个“安装到本机即可使用”的工具形应用：用户无需 Docker 即可在本地启动、配置、升级与维护，并尽量降低对外部服务的硬依赖。

该改动属于**架构级**与**交付形态**的重大调整：不仅涉及运行方式（容器→本地进程），还会影响配置方式、数据目录、存储方案、认证方案、构建与发布流水线，以及文档与运维策略。

## Why
1. **本地可用性**：Docker 对部分用户存在门槛（安装、权限、网络、磁盘占用、性能），且容器内执行工具调用无法天然访问宿主机环境（例如 repo、工具链、凭据、系统字体/二进制依赖）。
2. **工具属性更强**：oneAgent 的“工具调用 / shell / 编辑”天然贴近宿主机环境，作为本地工具可以提供更自然的使用体验（例如直接对本地代码仓进行 `rg`/`edit`/`bash`）。
3. **交付更轻**：提供一个可执行文件 + 配置目录 + 数据目录，降低启动复杂度，利于个人开发者与小团队快速上手。
4. **更清晰的分层**：将“开发模式（dev）”、“本地工具模式（local tool）”、“服务化部署（server）”明确分层，避免把容器当作唯一运行方式。

## 现状 (Current State)
以仓库当前内容为基线：
- 通过 `docker-compose.yml` 启动 `postgres`（pgvector 镜像）与 `backend`（自定义镜像，内含前端构建产物与大量系统依赖）。
- `backend` 以 Go/Gin 方式运行，启动时按 `DATABASE_URL` 连接 PostgreSQL；若未设置则降级为“无数据库运行”。
- API 默认启用后端签发 JWT（`APIJWTAuth`），并通过 `/api/auth/config` + `/api/auth/callback` 与 Keycloak 进行 OAuth 授权码交换，再签发本地 JWT。
- 前端为 Vue 3 SPA，最终静态文件被内嵌到后端二进制（`//go:embed static/*`），由后端统一对外服务。
- Docker 镜像中显式安装了 `ripgrep`/`jq`/`pandoc`/`ffmpeg`/`wkhtmltopdf`/`texlive` 等，用于工具调用链路的能力补全。

## 目标 (Goals)
### 核心目标
- **G1. 本地可执行交付**：提供一个“可安装、可升级”的本机可执行文件（例如 `oneagent`），不依赖 Docker 即可运行。
- **G2. 本地默认可用**：默认配置下尽量做到“零外部依赖即可跑起来”（至少不强依赖容器）；若确需外部依赖，则提供清晰的降级与提示。
- **G3. 可观测/可维护**：提供标准化的数据目录、日志目录、配置文件；提供 `doctor`（自检）与 `migrate/backup`（数据维护）能力。
- **G4. LAN 优先（约定）且安全可控**：本地模式默认面向局域网（LAN）使用（便于手机/同网段设备访问）。默认不依赖“按 IP 阻止公网访问”的策略（IPv6/反代场景下不可靠），而是默认启用一个自动生成的本地访问令牌（不过期）作为访问控制，并在登录页面明确提示“仅建议在可信局域网内使用；公网暴露风险极大”。
- **G5. 保留 Settings 中配置 token 的操作**：保留现有 Settings 页面/接口中对 LLM Provider API Key、搜索 API Key 等敏感 token 的配置与持久化能力（默认落在本地 SQLite）。

### 次级目标
- **G6. 向后兼容**：尽量保持现有 API 与前端交互方式；保留通过环境变量启动的能力；为现有 Docker 用户提供平滑迁移路径（至少一个过渡期）。
- **G7. 多运行画像（精简）**：支持：
  - `local`：本机单用户/局域网使用（默认）
  - `dev`：开发者体验（本地前后端分离开发、热更新等）
  - `server`：**废弃**（不再作为目标画像，不提供生产化能力）

## 非目标 (Non-Goals)
- 不在本提案中重写前端框架、替换 Gin、或进行大规模 UI 重构。
- 不承诺在第一阶段彻底移除 Docker 相关文件（Docker 仍可作为可选开发/CI 构建手段）。
- 不在第一阶段实现“完全离线安装全部系统依赖”（如 `pandoc`/`ffmpeg`/`wkhtmltopdf` 等可通过自检提示用户安装；后续再评估是否内置/打包）。

## What Changes
> 采用“分阶段交付”，但每一阶段都以可发布、可诊断、可回滚为标准（工业级交付）。

### 阶段 1: 本地工具交付版（可上线）
- 增加 `oneagent` CLI 入口（或将现有 `cmd/server` 扩展为多子命令）。
- 引入“home 目录”概念（默认 `~/.oneagent_default/`；选择 workspace 时 `home=<workspace>/`），作为 **agent 可修改文件的最大范围**。
- oneAgent 的可变状态统一存放在 `<home>/.oneagent/` 下，包含：
  - 配置文件（如 `config.yaml`）
  - 本地访问令牌（`config/auth_token`）
  - Settings SQLite（仅用于 Settings，例如 API keys）
  - 其它状态的文件存储（会话/trace/logs 等），并建议以 `session_id` 分目录/分文件以降低并发写冲突
- 默认以本机进程运行后端（并继续内嵌前端静态文件），通过 `oneagent serve` 启动；默认以 LAN 可访问方式启动，但不默认开放公网。
- 默认以“本地访问令牌（自动生成、不过期）”作为访问控制（替代 Keycloak/OAuth 登录）。
- 保存每次 LLM 调用的完整 request/response（含 messages）到日志文件（替代原先入库的做法）；trace 仅保存指标摘要与日志指针，便于成本分析与回溯排障。
- 为“外部依赖”提供 `oneagent doctor` 自检（例如检查 `rg`/`git`/`jq` 等可选工具是否存在，并输出建议；当 `rg` 缺失时提示安装，同时允许相关能力降级为 `grep -R`）。

### 阶段 2: 存储与认证的本地化（持续降依赖）
- 存储方案：
  - **SQLite 仅用于 Settings 持久化**（自动创建/迁移；Provider API Key 等敏感配置仅以 `has_*` 形式对外暴露）。
  - 其它状态（会话/消息/trace/subagent logs 等）走文件存储，落在 `<home>/.oneagent/data/` 与 `<home>/.oneagent/logs/`。
- 认证方案：引入 `AUTH_MODE=token`（默认），启动时自动生成一个不过期 token 并持久化到 `ONEAGENT_HOME`；可选 `AUTH_MODE=none` 仅用于开发/离线极简场景（需显式开启）。
- 本阶段不再支持 Postgres / `DATABASE_URL` 路径，以降低交付与维护成本。

### 阶段 3: 交付与升级体验（工具化完成度）
- 引入发布流水线（例如 GoReleaser）：
  - 产出 macOS/Linux 的可执行文件（本阶段不支持 Windows）
  - 产出校验与版本信息
  - 产出可选的 Homebrew 安装方式（可后置）
- 引入 `oneagent upgrade`（可选）或文档化升级流程。
- 明确 Docker 相关内容的定位：仅用于 CI 构建或“服务化部署参考”，而不是默认运行方式。

## 破坏性变更 (Breaking Changes / Risks)
该提案可能引入以下破坏性变化（具体以阶段划分控制风险）：
- **运行方式变化**：主推荐路径从 `docker-compose up` 变为 `oneagent serve`。
- **默认存储变化**：本阶段不再支持 `DATABASE_URL`/Postgres；默认使用文件存储（会话/trace/logs）+ Settings SQLite（仅配置）。
- **默认认证变化（本地访问令牌）**：不再使用 Keycloak/OAuth 登录；前端需要以 token 方式进行访问控制（对齐现有 `Authorization` header 的使用方式）。
- **运行画像变化（server 废弃）**：不再提供面向公网/多用户的 server 画像；默认面向局域网场景。
- **home 目录与权限**：home=agent 可修改文件的最大范围；超出 home 的写/改/删必须被拒绝，避免越权读写。

## Impact
- 代码：
  - 后端：启动入口、配置加载、Settings 持久化层（SQLite）与文件存储层、认证层、工具依赖检查
  - 前端：登录流程与配置获取流程可能需要适配新的 auth mode
  - 构建：本地构建脚本与 CI 发布流程
- 文档：
  - README（Quick Start）、部署文档（容器默认→可选）
  - 本地工具使用手册（安装/配置/自检/数据维护）

## 成功标准 (Success Criteria)
- 用户在 macOS/Linux（至少其一）上：
  - 不安装 Docker，也能通过单条命令启动并访问 UI（默认本地模式）
  - 局域网内其它设备可以访问（默认 LAN 监听），并在 UI/文档中明确提示“不要暴露到公网”（不依赖 IP 阻断，依赖 token 认证与使用约定）
  - 登录页面明确提示“仅建议在局域网内使用/公网风险大”，并通过本地访问令牌完成一次登录
  - 通过 Settings 页面完成一次 LLM Provider API Key 配置与一次搜索 API Key 配置，重启后仍生效
  - 能明确地完成最小配置（LLM provider 等）并进行一次成功对话
  - `oneagent doctor` 能给出可执行的依赖提示
- 现有 Docker 用户：
  - 仍可继续使用 Docker 路径（至少在一个过渡期内）
  - 或能清晰迁移到本地工具模式（文档与工具支持）

## 默认约定 (Defaults)
1. **反向代理与 IPv6**：默认 `local` 监听 `0.0.0.0`，同时支持 `--bind ::` 启用 IPv6；系统不依赖 RemoteIP 做安全阻断。反代下对 `X-Forwarded-*` 的信任由显式配置控制（例如 `TRUST_PROXY`），默认不信任。
2. **日志清理与轮转策略**：日志按日期目录分层；默认保留 30 天（可配置），启动时执行 best-effort 清理；`doctor` 输出日志目录与保留策略（避免磁盘无限增长）。
3. **敏感 token 的落盘策略**：不做落盘加密（避免引入密钥管理复杂度）；依赖本机文件权限，并确保 API 永不返回明文 token（仅返回 `has_*`）。
