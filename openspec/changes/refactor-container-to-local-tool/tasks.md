# 任务列表 (Tasks): 容器运行 → 本地工具形应用

> 说明：本文件用于实现阶段（apply）逐条勾选。每一项都应可验证（含测试/自检/手工步骤）。

## 1. 对齐与基线梳理
- [ ] 明确目标画像与范围（local/dev；server 废弃），并在 proposal/design 中锁定关键决策（默认 home=`~/.oneagent_default`、workspace=home、去掉 sandbox、Settings SQLite + 其它文件存储、scope glob、本地访问令牌、IPv6/反代立场、不支持 Windows） <!-- id: 1 -->
- [ ] 盘点当前 Docker 镜像内置依赖清单（rg/jq/pandoc/ffmpeg/...）并分类：必须/可选/可移除 <!-- id: 2 -->

## 2. CLI 入口与运行画像
- [ ] 新增 `oneagent` CLI 入口（或扩展现有 `cmd/server`）并提供 `--help`/`--version` <!-- id: 3 -->
- [ ] 实现 `oneagent serve --profile {local,dev}` 与基础参数（port、bind、home）（可选：`server` 作为废弃别名仅输出提示并按 local 处理） <!-- id: 4 -->
- [ ] 实现配置优先级：flags > env > config file > defaults，并补齐文档 <!-- id: 5 -->

## 3. 本地 Home/Data 目录
- [ ] 设计并实现 `ONEAGENT_HOME`（或 `--home`）与默认路径解析：未指定时默认 `~/.oneagent_default`；若会话选择了 workspace 且未显式指定 `--home`，则 `ONEAGENT_HOME=<workspace>/` <!-- id: 6 -->
- [ ] 实现目录结构初始化：`ONEAGENT_HOME/.oneagent/{config,data,logs,tmp}`（最小权限） <!-- id: 7 -->

## 3.5 Workspace（Project）与工具作用域
- [ ] 明确定义 workspace：新建会话时可选择/复用 workspace；不启用 workspace 时默认只对话不改文件 <!-- id: 26 -->
- [ ] 将文件类工具的默认可写范围对齐到 workspace（workspace 外允许读取任意绝对路径，但默认不允许写/改/删；越界写/改/删返回清晰错误） <!-- id: 27 -->
- [ ] 重新定义 `BASH_ROOT_DIR`：默认对齐到当前会话的 workspace（并保留 env/flag 覆盖能力） <!-- id: 28 -->

## 4. 存储层（Settings SQLite + 文件存储；不支持 Postgres）
- [ ] 定义 Settings 的最小 schema（仅覆盖 Settings 需要的数据），并实现迁移机制 <!-- id: 9 -->
- [ ] Settings 默认落 SQLite：`ONEAGENT_HOME/.oneagent/settings.db`（自动创建/迁移） <!-- id: 10 -->
- [ ] 会话/消息/trace 等其它状态使用文件存储：`ONEAGENT_HOME/.oneagent/data/` 与 `ONEAGENT_HOME/.oneagent/logs/` <!-- id: 11 -->
- [ ] 增加最小集成测试：Settings SQLite 读写 + 文件存储 round-trip + health/doctor 可诊断 <!-- id: 12 -->

## 5. 认证模式（本地访问令牌；无登录体系）
- [ ] 增加 `AUTH_MODE` 配置（token/none；兼容 password 作为别名）并在中间件层实现分支 <!-- id: 13 -->
- [ ] 实现 Token Auth：启动时自动生成不过期 token 并持久化到 `ONEAGENT_HOME/.oneagent/config/auth_token`；校验 `Authorization: Bearer <token>` <!-- id: 14 -->
- [ ] 前端适配：以“token”作为访问凭证（可复用现有 token 存储逻辑），登录页明确提示“仅建议局域网/公网风险大” <!-- id: 15 -->
- [ ] 兼容处理：Keycloak/OAuth 相关接口与前端路由标记为废弃并逐步移除（文档与代码都需对齐） <!-- id: 16 -->

## 6. Doctor 自检与可选依赖
- [ ] 实现 `oneagent doctor`：输出版本、profile、home 路径、端口、存储可用性、关键二进制可用性 <!-- id: 17 -->
- [ ] 对缺失依赖给出平台化安装建议（macOS: brew；Linux: apt/yum） <!-- id: 18 -->

## 7. 构建与发布（替代 Docker 作为默认交付）
- [ ] 增加本地构建脚本（Makefile/justfile）统一 `build-frontend` + `build-backend` + embed 流程 <!-- id: 19 -->
- [ ] 增加 release pipeline（可选：GoReleaser），产出 macOS/Linux 二进制与 checksum <!-- id: 20 -->
- [ ] 更新 README 与 docs：默认运行方式改为本地工具，并将 Docker 标记为可选方案 <!-- id: 21 -->

## 8. 端到端验证（TDD）
- [ ] E2E：在无 Docker 情况下从零启动（local profile），使用本地访问令牌完成一次登录并完成一次对话 <!-- id: 22 -->
- [ ] E2E：缺失可选依赖（如 rg）时，doctor 与工具调用表现符合预期（降级/提示） <!-- id: 23 -->
- [ ] E2E：在局域网内通过其它设备访问 UI（默认 LAN 监听），登录页包含“仅建议局域网/公网风险大”提示（不依赖 IP 阻断） <!-- id: 24 -->
- [ ] E2E：Settings 页面可配置 LLM Provider API Key 与搜索 API Key，重启后仍生效，且接口不会泄露明文 token <!-- id: 25 -->
