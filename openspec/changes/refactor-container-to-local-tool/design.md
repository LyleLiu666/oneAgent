# 设计: 将 oneAgent 从“容器运行”改造为“本地工具形应用”

## 背景与约束 (Context)
oneAgent 当前以 Web 应用形态运行：后端（Go/Gin）对外提供 UI 静态资源与 API，前端（Vue 3）通过 SSE 获取流式响应；后端具备“工具调用能力”，并且这些工具（例如 `rg`/`bash`）在容器镜像中通过安装系统依赖来确保可用。

把它改造成“本地工具”，意味着：
- 运行环境从“受控容器”变为“用户机器的真实环境”，能力更强但不确定性更高（依赖是否存在、路径权限、平台差异）。
- 工具调用将更贴近用户真实工作目录，但安全边界更重要（尤其是 `bash` 能力）。
- 交付与升级不再依赖 Docker build，而需要一个明确的 release pipeline。

## 目标 / 非目标 (Goals / Non-Goals)
### Goals
- 提供一个可执行文件（`oneagent`）作为统一入口。
- 提供“本地默认可用”的运行画像（local profile），并以局域网（LAN）为默认使用场景。
- `server` profile 作为**废弃画像**处理：不再提供面向公网/多用户的生产化能力。
- 提供更明确的配置与数据目录规范，便于运维与排障。
- 保留 Settings 中配置 token 的操作，并确保在本地 SQLite 下可持久化。

### Non-Goals
- 不做大型 UI/UX 重做（除适配本地登录/配置所需页面/流程）。
- 不在第一阶段强制打包所有系统依赖（例如 `pandoc/ffmpeg`），优先做“可检测、可提示、可降级”。

## 关键设计决策 (Decisions)
本提案将把“运行画像”作为一等概念，避免一刀切：

### 1) Profile: `local` vs `dev`（`server` 废弃）
- `local`（默认）：面向本机单用户/家庭或办公室局域网使用。
  - 默认监听 `0.0.0.0`（便于同网段设备访问）
  - 默认阻止公网访问（通过来源 IP 私网白名单/显式开关）
  - 默认使用本地数据目录，默认 SQLite 持久化
  - 默认使用共享密码作为访问控制（不再依赖 OAuth/Keycloak）
- `dev`：开发模式（前后端分离、热更新、可选 mock）。
- `server`：**废弃**（不再作为目标画像；CLI 可选择保留兼容入口但仅输出废弃提示并按 local 处理）。

推荐实现方式：
- `oneagent serve --profile local|dev`（可选兼容 `server` 但仅作为废弃别名）
- 配置文件也可设置 `profile`，CLI flag 优先级最高。

### 2) 数据目录（Data Dir）与可变状态管理
容器时代通过 volume 挂载实现“可变状态外置”。本地工具需要一个一致的目录布局。

建议目录：
- macOS：`~/Library/Application Support/oneagent/`
- Linux：`~/.local/share/oneagent/`（或遵循 XDG）
- Windows：`%APPDATA%\\oneagent\\`

目录下建议结构：
```
oneagent/
  config/            # 配置（用户可编辑）
    config.yaml
  data/              # 数据（数据库、索引、缓存）
    oneagent.db      # SQLite（若启用）
  sandbox/           # bash-root（工具沙箱根目录）
  logs/              # 日志
  tmp/               # 临时文件（可清理）
```

关键点：
- **不要**默认把数据落在当前工作目录，避免污染用户 repo。
- 允许通过 `ONEAGENT_HOME` 或 `--home` 显式覆盖（便于便携/多实例）。
- `doctor` 输出当前实际 home 路径与关键子路径。

### 3) 配置层级与兼容策略
现状主要依赖环境变量。工具化后应提供配置文件，同时保留 env 兼容。

建议优先级（从高到低）：
1. CLI flags（`--port` / `--db` / `--auth-mode` / `--home`）
2. 环境变量（现有 `PORT`/`DATABASE_URL`/`KEYCLOAK_*` 等继续支持）
3. 配置文件（`config.yaml`）
4. 内置默认值

为降低破坏性：
- 现有环境变量键名尽量不变。
- 新增配置项时，同时提供 env 对应（例如 `AUTH_MODE`、`ONEAGENT_HOME`）。

### 4) 存储：默认 SQLite + 可选 Postgres（推荐）
容器默认提供 Postgres，但本地工具如果仍强依赖 Postgres，会显著降低“开箱即用”。

建议：
- 默认：SQLite（本地文件）作为本地 profile 的默认持久化。
- 可选：当设置 `DATABASE_URL`（或 `--database-url`）时，使用 Postgres，作为迁移/高级用户路径（不再绑定 server profile 语义）。

实现注意：
- 当前模型使用 `jsonb` 等 Postgres 特性（GORM `type:jsonb`），需要评估 SQLite 兼容或改造（例如统一为 `TEXT` 存 JSON）。
- 迁移策略：优先做到“SQLite 与 Postgres 均可跑通 + 自动迁移”，至于“跨库数据迁移”可后置为增强项。

### 5) 认证：新增 Local Auth（避免强依赖 Keycloak）
当前 OAuth 流程依赖 Keycloak，且前端需要 Keycloak 配置进行跳转。为满足“本地工具 + 局域网默认可访问”的体验，本提案采用**共享密码**作为访问控制，替代登录体系。

推荐方案：Password Auth（共享密码）
- `AUTH_MODE=password`（默认）：
  - 后端通过环境变量/配置文件读取一个共享密码（不做账号体系）
  - 前端首次访问时输入密码，并将其保存为“访问凭证”（例如存储到本地并在每个请求中带上 `Authorization` 头）
  - 后端在中间件中验证密码后放行请求，并注入一个固定的单用户身份（例如 `user_id="local"`），以复用现有数据模型与 Settings/Session 存储逻辑
- `AUTH_MODE=none`（可选）：
  - 仅用于开发/离线极简场景，必须显式开启
  - 启动日志中输出强提示，并建议只在可信网络中使用

Keycloak/OAuth 相关能力：
- 在 `server` profile 废弃后，Keycloak/OAuth 不再作为主路径；可选择保留兼容接口一段时间，但应明确标记为废弃并计划移除。

### 6) 工具依赖与 `doctor`
容器中通过 apt 安装了大量二进制依赖，本地工具无法保证这些依赖存在。

设计原则：
- “能降级就降级”：例如 `rg` 不存在时，`rg` 工具返回 `available=false`（当前已实现），并提示可用替代策略。
- “能提示就提示”：`oneagent doctor` 检查关键依赖并给出安装建议（按平台输出）。

建议 `doctor` 至少包含：
- 运行环境：OS/Arch、版本号、数据目录路径、profile
- 关键二进制：`git`、`rg`、`jq`、`bash`、（可选）`pandoc`、`ffmpeg`、`wkhtmltopdf`
- 端口占用检测：默认端口是否可用
- 存储可用性：SQLite 文件是否可创建/是否可写；Postgres URL 是否可连通（可选）

### 7) 发布与构建流水线
当前 Dockerfile 负责“构建前端 + 构建后端 + 打包系统依赖”。本地工具需要新的 pipeline：

- 本地开发构建（从源码）：
  - `pnpm build` 产出 `frontend/dist`
  - `go build` 将 `frontend/dist` 复制到 `backend/cmd/server/static` 或类似目录后 embed
- Release 构建：
  - CI 内完成前端构建
  - 产出多平台二进制
  - 产出 `checksums.txt` 与版本信息

建议引入：
- `Makefile`（或 `justfile`）统一本地构建命令（可选）
- GoReleaser（跨平台发布）（可选，但推荐）

## 迁移方案 (Migration Plan)
### 迁移路径 A（过渡期）：继续使用 Docker（不阻断老用户）
- 对已有 Docker 用户，在过渡期内保留 `docker-compose.yml` 路径。

### 迁移路径 B（推荐目标）：本地工具默认 SQLite + Password Auth（LAN 优先）
- 新用户：默认 SQLite + 共享密码，开箱即用；局域网可访问但默认阻止公网访问。
- 老用户：可以选择继续使用 Postgres（仅作为迁移/高级用户路径），不强制迁移；后续再提供“导入/导出”工具。

回滚策略：
- 保留 Docker 运行方式（至少一个过渡期）。
- 保留 Postgres 作为可选后端存储。

## 风险与权衡 (Risks / Trade-offs)
- **SQLite 兼容性**：当前模型包含 Postgres 特性，需要兼容层或模型调整。
- **本地安全**：`bash` 工具在宿主机上运行风险更高；需要更清晰的沙箱策略与默认限制（例如只允许在 `BASH_ROOT_DIR` 内运行、限制命令、限制资源）。
- **发布复杂度**：从 Docker build 转为多平台 release，需要新增 CI 产物、签名与版本治理。
- **支持成本**：本地环境的差异会带来更多问题（依赖缺失、权限、路径、编码、字体等）。

## 开放问题 (Open Questions)
- “工具形应用”的核心交互是否仍以 Web UI 为主？是否需要增加纯 CLI 模式（例如 `oneagent chat`）？
- 共享密码的配置与轮换机制：仅 env/config，还是支持首次启动生成/交互式设置？
- LAN 与公网的判定策略：默认私网白名单是否覆盖 IPv6 ULA/链路本地地址？是否需要显式 `--allow-public`？
- SQLite 数据库中落盘的 API Key 是否需要加密（例如 password 派生密钥），还是仅依赖本机文件权限即可？
