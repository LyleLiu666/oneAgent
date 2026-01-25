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

### 登录方式（本地访问令牌）

- 默认 `AUTH_MODE=token`
- token 文件路径：`ONEAGENT_HOME/.oneagent/config/auth_token`
- 运行 `./dist/oneagent doctor` 可查看 token 文件路径（不会输出明文 token）
- 打开 UI 的登录页，粘贴 token 完成登录

安全提示：仅建议在可信局域网内使用；将服务暴露到公网风险极大。

## Data Layout

- `ONEAGENT_HOME`（默认 `~/.oneagent_default`）是 agent 可修改文件的最大范围
- oneAgent 的内部状态统一落在：`ONEAGENT_HOME/.oneagent/`
  - `config/`：配置与 `auth_token`
  - `settings.db`：Settings（SQLite，仅用于敏感 token 等配置）
  - `data/sessions/`：会话与消息（按 `session_id` 分目录/分文件）
  - `logs/llm/`：每次 LLM 调用完整 request/response 日志（按日期目录）

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ONEAGENT_HOME` | Home 目录（agent 可写边界） | `~/.oneagent_default` |
| `PROFILE` | `local` / `dev` | `local` |
| `BIND` | 监听地址 | `0.0.0.0`（local） |
| `PORT` | 端口 | `8080` |
| `AUTH_MODE` | `token` / `none` | `token` |
| `ENABLE_TRACE` | 是否启用 trace | `false` |
| `BASH_ROOT_DIR` | bash/文件工具默认根目录 | `ONEAGENT_HOME` |
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
