# 部署指南（本地工具默认）

> 本项目默认以“本地工具形应用”交付：运行单个 `oneagent` 二进制即可（不依赖 Postgres/Keycloak）。

---

## 1. 推荐方式：本地工具启动

```bash
make build
./dist/oneagent serve
```

访问：`http://localhost:8080`

认证：
- 默认 `AUTH_MODE=token`
- token 文件：`ONEAGENT_HOME/.oneagent/config/auth_token`
- 运行 `./dist/oneagent doctor` 查看 token 文件路径（不会输出明文 token）

安全提示：仅建议在可信局域网内使用；将服务暴露到公网风险极大。

---

## 2. 环境变量（常用）

| 变量 | 描述 | 默认值 |
|------|------|--------|
| `ONEAGENT_HOME` | Home 目录（agent 可写边界） | `~/.oneagent_default` |
| `PROFILE` | `local`/`dev` | `local` |
| `BIND` | 监听地址 | local=`0.0.0.0` |
| `PORT` | 端口 | `8080` |
| `AUTH_MODE` | `token`/`none` | `token` |
| `BASH_ROOT_DIR` | bash/文件工具根目录 | `ONEAGENT_HOME` |
| `LOG_RETENTION_DAYS` | 日志保留天数 | `30` |

---

## 3. Legacy：Docker Compose（不再作为默认路径）

本项目已不再以 Postgres/Keycloak 作为默认依赖，也不再推荐以 Docker Compose 作为“默认交付形态”。
如果你需要容器化部署，请以本地工具模式的目录契约与认证策略为准，自行在反代/容器编排中落地（并明确公网暴露风险）。
