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
- 保留 Settings 中配置 token 的操作，并确保 Settings 可持久化（SQLite），其它状态使用文件存储。

### Non-Goals
- 不做大型 UI/UX 重做（除适配本地登录/配置所需页面/流程）。
- 不在第一阶段强制打包所有系统依赖（例如 `pandoc/ffmpeg`），优先做“可检测、可提示、可降级”。

## 关键设计决策 (Decisions)
本提案将把“运行画像”作为一等概念，避免一刀切：

### 1) Profile: `local` vs `dev`（`server` 废弃）
- `local`（默认）：面向本机单用户/家庭或办公室局域网使用。
  - 默认监听 `0.0.0.0`（便于同网段设备访问）
  - 默认不依赖“按客户端 IP 阻断公网访问”的策略（IPv6/反代场景下不可靠），而是默认启用本地访问令牌认证，并在 UI 中明确风险提示
  - 默认使用文件存储保存会话/trace 等状态（不依赖数据库）；Settings 单独使用 SQLite 持久化
  - 默认使用本地访问令牌（自动生成、不过期）作为访问控制（不再依赖 OAuth/Keycloak）
- `dev`：开发模式（前后端分离、热更新、可选 mock）。
- `server`：**废弃**（不再作为目标画像；CLI 可选择保留兼容入口但仅输出废弃提示并按 local 处理）。

推荐实现方式：
- `oneagent serve --profile local|dev`（可选兼容 `server` 但仅作为废弃别名）
- 配置文件也可设置 `profile`，CLI flag 优先级最高。

### 2) 数据目录（Data Dir）与可变状态管理
容器时代通过 volume 挂载实现“可变状态外置”。本地工具需要一个一致的目录布局。

约定：
- **默认 ONEAGENT_HOME**：`~/.oneagent_default`（仅 macOS/Linux；本阶段不支持 Windows），用于承载 oneAgent 的内部状态目录。
- **workspace（项目目录）**：会话级可选，用于定义工具默认作用域与写入边界（“agent 可修改文件的最大范围”）。
- oneAgent 的内部可变状态统一存放在 `ONEAGENT_HOME/.oneagent/` 下，避免把 `config/ logs/ data/` 直接散落到 workspace 根目录。

目录结构建议：
```
<home>/
  .oneagent/
    config/                 # 配置（用户可编辑）
      config.yaml
      auth_token            # 本地访问令牌（opaque string）
    settings.db             # SQLite：仅用于 Settings（API keys 等）
    data/                   # 文件存储：会话/消息等（按 session_id 分目录/分文件）
      sessions/
        <session_id>/
          session.json
          messages.jsonl
    logs/                   # 日志：trace/llm/subagent 等（按日期 + session_id 分层）
      trace/YYYY-MM-DD/<session_id>/trace.jsonl
      llm/YYYY-MM-DD/<session_id>/<call_id>.json
      subagent/YYYY-MM-DD/<session_id>/<run_id>/...
    tmp/                    # 临时文件（可清理）
    skills/                 # 项目私有 skills（若使用）
    PLAN.md                 # 计划文件（若启用 plan 模块）
```

关键点：
- **workspace = agent 可修改文件的最大范围**：任何写/改/删默认必须在 workspace 内；workspace 外允许读取任意绝对路径，但不允许写/改/删。
- 允许通过 `--home` 或 `ONEAGENT_HOME` 显式覆盖内部状态目录位置；如需“项目私有数据”，用户可将 `ONEAGENT_HOME=<workspace>`（但本阶段不强制自动切换）。
- `doctor` 输出当前实际 `ONEAGENT_HOME` 路径与关键子路径（但不得输出明文 token）。

### 2.5) Workspace（Project）与工具作用域
容器时代的“工作目录”主要由 volume 与容器文件系统决定；本地工具需要一个更明确的 project 边界。本提案将 `workspace` 定义为“本次会话/任务的项目根目录”：
- 用户在新建会话时可选择是否启用 workspace，并可复用已存在的 workspace（更像 coding 场景下的 project 选择）
- 默认仅允许修改 workspace 内文件；workspace 外允许读取任意绝对路径，但原则上避免写入
- 文件类工具/搜索类工具/命令执行工具应默认对齐到 workspace（并支持按子目录进一步收敛为 scope，以支持未来并发 subagent）

> 已知限制：`bash/run_command` 在宿主机上运行，无法完全防止绕过文件工具层的 workspace/scope 约束。出于灵活性与实现成本考虑（也无法彻底防止通过脚本/编辑器修改文件），当前不做硬性拦截，仅做强引导：默认 `BASH_ROOT_DIR` 对齐到当前会话的 workspace（并保留 env/flag 覆盖能力），并在提示词/错误信息中强调“优先用文件工具修改文件；bash 主要用于只读/运行命令”。

### 3) 配置层级与兼容策略
现状主要依赖环境变量。工具化后应提供配置文件，同时保留 env 兼容。

建议优先级（从高到低）：
1. CLI flags（`--port` / `--bind` / `--auth-mode` / `--home`）
2. 环境变量（现有 `PORT`/`KEYCLOAK_*` 等继续支持；`DATABASE_URL` 在本阶段不再支持）
3. 配置文件（`config.yaml`）
4. 内置默认值

为降低破坏性：
- 现有环境变量键名尽量不变。
- 新增配置项时，同时提供 env 对应（例如 `AUTH_MODE`、`ONEAGENT_HOME`）。

### 4) 存储：Settings 用 SQLite，其余用文件存储（推荐）
本阶段不再支持 Postgres，也不以 SQLite 作为“通用持久化”。原则是：
- **SQLite 只用于 Settings**（例如 LLM Provider API Key、搜索 API Key 等敏感配置的持久化与 `has_*` 查询）。
- **其它状态一律走文件存储**（例如会话/消息/trace/subagent logs），落在 `<home>/.oneagent/data/` 与 `<home>/.oneagent/logs/`。

好处：
- 降低交付复杂度（不引入外部数据库，也避免跨平台 SQLite driver/FTS 兼容的额外负担）。
- 更符合“工具形应用”的直觉：大部分产物（日志、trace、findings）天然就是文件。

实现注意：
- Settings SQLite 的 schema 要尽量简单、可迁移（避免依赖数据库高级特性）。
- 文件存储需要确定：目录结构、文件命名（按 session_id/run_id）、并发写入策略、以及向后兼容/迁移策略。

### 5) 认证：新增 Local Auth（避免强依赖 Keycloak）
当前 OAuth 流程依赖 Keycloak，且前端需要 Keycloak 配置进行跳转。为满足“本地工具 + 局域网默认可访问”的体验，本提案采用**本地访问令牌**作为访问控制，替代登录体系。

推荐方案：Token Auth（本地访问令牌）
- `AUTH_MODE=token`（默认）：
  - 后端在启动时读取/生成一个本地访问令牌（不过期、随机字符串，不要求 JWT 结构），并持久化到固定路径 `ONEAGENT_HOME/.oneagent/config/auth_token`
  - 前端首次访问时输入 token，并将其保存为“访问凭证”（例如存储到本地并在每个请求中带上 `Authorization: Bearer <token>`）
  - 登录页面必须明确提示：仅建议在可信局域网内使用；将服务暴露到公网风险极大
  - 后端在中间件中验证 token 后放行请求，并注入一个固定的单用户身份（例如 `user_id="local"`），以复用现有数据模型与 Settings/Session 存储逻辑
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
- 存储可用性：`<home>/.oneagent/settings.db` 是否可创建/是否可写；`<home>/.oneagent/data/` 是否可写
- 认证：输出 token 文件路径（例如 `<home>/.oneagent/config/auth_token`），但不输出明文 token

#### Docker 镜像依赖盘点（历史默认容器）
> 目的：把容器内“预装依赖”拆成三类：**必须**（核心能力直接依赖）、**可选**（能力增强/特性依赖）、**可移除**（仅 build 阶段需要或对运行期无贡献）。

| 依赖 | Dockerfile 安装位置 | 用途/关联能力 | 分类 |
| --- | --- | --- | --- |
| `bash` | Ubuntu base | `bash`/`run_command` 工具执行 | 必须 |
| `ca-certificates` | basic utilities | HTTPS（LLM provider / 更新等） | 必须 |
| `git` | basic utilities / build stage | repo 操作（常见工作流） | 必须 |
| `tzdata`/`locales` | basic utilities | 运行环境一致性（时间/编码） | 可选 |
| `ripgrep` (`rg`) | basic utilities | 本地全文搜索加速（缺失可降级 `grep -R`） | 可选 |
| `jq` | basic utilities | JSON 处理（工具链/脚本常用） | 可选 |
| `curl`/`wget`/`gnupg` | basic utilities / nodesource | 下载/安装脚本（偏运维） | 可移除（runtime 镜像） |
| `zip`/`unzip`/`tree`/`procps`/`nano` | basic utilities | 辅助调试/运维/编辑 | 可选（多数可不装） |
| `build-essential`/`pkg-config`/`libssl-dev`/`libffi-dev` | build deps | 构建 Python 依赖/编译扩展 | 可移除（runtime 镜像） |
| `nodejs` | app deps | 运行期若无 Node 工具链则不需要 | 可移除（runtime 镜像） |
| `python3`/`pip`/`venv` + Python libs（`pypdf`/`pdfminer.six`/`opencv`/`librosa` 等） | app deps + pip | 文档/多媒体处理类扩展能力（若有相关工具链） | 可选 |
| `pandoc`/`poppler-utils` | app deps | 文档转换/PDF 工具链 | 可选 |
| `ffmpeg`/`pydub` | app deps + pip | 音视频处理 | 可选 |
| `wkhtmltopdf`/`texlive-latex-base`/fonts | app deps | HTML/PDF/LaTeX 输出与字体 | 可选（体积大） |

说明：
- 本地工具形态默认不再“内置”上述依赖；改为 `doctor` 诊断 + 缺失提示 + 尽可能降级（例如 `rg → grep -R`）。
- 对于 Docker（遗留/可选路径），建议后续再做一轮“runtime 镜像瘦身”：把 **可移除** 的依赖迁出最终镜像，仅保留构建阶段。

### 7) 发布与构建流水线
当前 Dockerfile 负责“构建前端 + 构建后端 + 打包系统依赖”。本地工具需要新的 pipeline：

- 本地开发构建（从源码）：
  - `pnpm build` 产出 `frontend/dist`
  - `go build` 将 `frontend/dist` 复制到 `backend/cmd/server/static` 或类似目录后 embed
- Release 构建：
  - CI 内完成前端构建
  - 产出 macOS/Linux 二进制（本阶段不支持 Windows）
  - 产出 `checksums.txt` 与版本信息

建议引入：
- `Makefile`（或 `justfile`）统一本地构建命令（可选）
- GoReleaser（跨平台发布）（可选，但推荐）

## 迁移方案 (Migration Plan)
### 迁移路径 A（过渡期）：继续使用 Docker（不阻断老用户）
- 对已有 Docker 用户，在过渡期内保留 `docker-compose.yml` 路径。

### 迁移路径 B（推荐目标）：本地工具默认文件存储 + Settings SQLite + Token Auth（LAN 优先）
- 新用户：默认文件存储（会话/trace/logs 等）+ Settings SQLite（仅配置）+ 本地访问令牌（自动生成、不过期），开箱即用；局域网可访问并明确提示不要暴露到公网（不做 IP 阻断）。
- 老用户：在过渡期内继续使用 Docker 路径；若需要迁移历史数据，后续以“导入/导出（文件）”的形式提供，而不是保留 Postgres 运行路径。

回滚策略：
- 保留 Docker 运行方式（至少一个过渡期）。
- 不提供 Postgres 作为可选后端存储（以保持交付与维护成本可控）。

## 风险与权衡 (Risks / Trade-offs)
- **本地安全**：`bash` 工具在宿主机上运行风险更高；需要更清晰的沙箱策略与默认限制（例如只允许在 `BASH_ROOT_DIR` 内运行、限制命令、限制资源）。
- **发布复杂度**：从 Docker build 转为多平台 release，需要新增 CI 产物、签名与版本治理。
- **支持成本**：本地环境的差异会带来更多问题（依赖缺失、权限、路径、编码、字体等）。
- **文件存储的演进成本**：一旦文件目录结构/命名被用户依赖，未来迁移会更难；需要尽早确定约定并尽量保持兼容。

## 默认约定 (Defaults)
- 核心交互以 Web UI 为主；本变更不引入纯 CLI 聊天模式（如 `oneagent chat`）。
- 反向代理支持以“显式信任”为原则：默认不信任 `X-Forwarded-*`，通过配置（如 `TRUST_PROXY`/allowlist）显式开启后再使用相关头部改善 URL/日志体验。
- Settings SQLite 中落盘的 API Key 不做加密；依赖本机文件权限，并确保 API 永不返回明文 token（仅返回 `has_*`）。
