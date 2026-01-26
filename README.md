# OneAgent

oneAgent 以“本地工具形应用”为默认交付：一个 `oneagent` 可执行文件即可启动（不依赖 Docker/Keycloak/Postgres）。

## Quick Start（本地工具默认路径）

### Prerequisites

- Go（需能构建 `backend/` 模块）
- Node.js + npm（用于构建前端）

### Build & Run

```bash
make build
./dist/oneagent serve
```

访问：`http://localhost:8080`

### Release（产物打包）

```bash
VERSION=v0.1.0 make release
ls dist/release
```

### 登录方式（本地访问令牌）

- 默认 `AUTH_MODE=token`
- token 文件路径：`ONEAGENT_HOME/.oneagent/config/auth_token`
- 运行 `./dist/oneagent doctor` 可查看 token 文件路径（不会输出明文 token）
- 打开 UI 的登录页，粘贴 token 完成登录

安全提示：仅建议在可信局域网内使用；将服务暴露到公网风险极大。

### Workspace（文件工具作用域）

默认新会话不启用 workspace（仅对话）。如需使用 `edit/write_file/rg/run_command` 等工具修改或搜索本地文件：
- 在 Chat 顶部输入 `Workspace path (server)`（服务运行机器上的绝对路径，例如你的项目目录）
- 后端会将其保存到 session metadata，并将文件工具/命令工具默认限制在该目录内

## Skills（技能）

oneAgent 的 skills 以“目录包 + `SKILL.md`”形式组织（可选包含 `scripts/`、`references/` 等）。

### 发现位置与优先级

oneAgent 自带一批内置 skills（随二进制发布，source=`.builtin`），同时也会从 workspace / 本机目录发现自定义 skills。

当会话启用 workspace 后，系统会按以下优先级发现并去重（同名/同 ID 先出现者生效）：
1) `<workspace>/.oneagent/skills/**/SKILL.md`
2) `<workspace>/skills/**/SKILL.md`（推荐：随仓库提交的项目 skills）
3) `<workspace>/.claude/skills/**/SKILL.md`（兼容 Claude Code 的项目内 skills）
4) `~/.claude/skills/**/SKILL.md`
5) `~/.codex/skills/**/SKILL.md`
6) `.builtin`（内置 skills，随二进制发布，path 形如 `builtin:skills/<id>/SKILL.md`）

### 验证 skills 是否被识别

```bash
cd backend
go run ./cmd/oneagent skills search --query "create-skill"           # 内置 skills（不依赖 workspace）
go run ./cmd/oneagent skills search --query "tech-spec-architect"    # 内置 skills（不依赖 workspace）
go run ./cmd/oneagent skills search --query "my-skill" --workspace "$(pwd)/.."   # workspace skills
```

## Data Layout

- `ONEAGENT_HOME`（默认 `~/.oneagent_default`）是 oneAgent 的内部状态目录根
- oneAgent 的内部状态统一落在：`ONEAGENT_HOME/.oneagent/`
  - `config/`：配置与 `auth_token`
  - `settings.db`：Settings（SQLite，仅用于敏感 token 等配置）
  - `data/sessions/`：会话与消息（按 `session_id` 分目录/分文件）
  - `logs/llm/`：每次 LLM 调用完整 request/response 日志（按日期目录）
- `workspace`（会话级可选）是文件工具/命令工具的默认作用域与写入边界；未设置时文件相关工具将提示 `workspace is not set`

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ONEAGENT_HOME` | 内部状态目录根（config/settings/data/logs） | `~/.oneagent_default` |
| `PROFILE` | `local` / `dev` | `local` |
| `BIND` | 监听地址 | `0.0.0.0`（local） |
| `PORT` | 端口 | `8080` |
| `AUTH_MODE` | `token` / `none` | `token` |
| `ENABLE_TRACE` | 是否启用 trace | `false` |
| `BASH_ROOT_DIR` | （可选）显式覆盖工具根目录（未设置时使用会话 workspace） | _unset_ |
| `LOG_RETENTION_DAYS` | 日志保留天数 | `30` |

注意：`DATABASE_URL`（Postgres）在本地工具模式下不支持，设置后会拒绝启动。

## Project Structure

```
oneAgent/
├── backend/
│   ├── cmd/oneagent/        # oneagent CLI (serve/doctor/...)
│   ├── cmd/server/          # legacy entry (wrapper)
│   ├── internal/
│   │   ├── config/          # Configuration management
│   │   ├── runtime/         # local runtime (layout/auth/settings/sessions/logs)
│   │   ├── settingsdb/      # SQLite settings store
│   │   ├── sessionstore/    # file-based sessions/messages
│   │   ├── handler/         # API handlers
│   │   ├── middleware/      # Auth middleware
│   │   └── web/             # embedded frontend assets
│   └── Dockerfile
├── frontend/
│   ├── src/
│   │   ├── api/             # API client
│   │   ├── components/      # Vue components
│   │   ├── composables/     # Vue composables
│   │   ├── router/          # Vue Router
│   │   ├── stores/          # Pinia stores
│   │   └── views/           # Page views
│   └── package.json
├── docker-compose.yml
└── .env.example
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check |
| `POST` | `/api/chat` | Streaming chat (SSE) |
| `GET` | `/api/sessions` | List chat sessions |
| `GET` | `/api/sessions/:id` | Get session with messages |
| `DELETE` | `/api/sessions/:id` | Delete session |
| `GET` | `/api/me` | Get current user info |
