# 任务列表 (Tasks): 容器运行 → 本地工具形应用

> 说明：本文件用于实现阶段（apply）逐条勾选。每一项都应可验证（含测试/自检/手工步骤）。

## 1. 对齐与基线梳理
- [x] 明确目标画像与范围（local/dev；server 废弃），并在 proposal/design 中锁定关键决策（默认 `ONEAGENT_HOME=~/.oneagent_default`、workspace 为会话级可选工具作用域、去掉 sandbox、Settings SQLite + 其它文件存储、scope glob、本地访问令牌、IPv6/反代立场、不支持 Windows） <!-- id: 1 -->
- [x] 盘点当前 Docker 镜像内置依赖清单（rg/jq/pandoc/ffmpeg/...）并分类：必须/可选/可移除 <!-- id: 2 -->

## 2. CLI 入口与运行画像
- [x] 新增 `oneagent` CLI 入口（或扩展现有 `cmd/server`）并提供 `--help`/`--version` <!-- id: 3 -->
- [x] 实现 `oneagent serve --profile {local,dev}` 与基础参数（port、bind、home）（可选：`server` 作为废弃别名仅输出提示并按 local 处理） <!-- id: 4 -->
- [x] 实现配置优先级：flags > env > config file > defaults，并补齐文档 <!-- id: 5 -->

## 3. 本地 Home/Data 目录
- [x] 设计并实现 `ONEAGENT_HOME`（或 `--home`）与默认路径解析：未指定时默认 `~/.oneagent_default`；workspace 为会话级工具作用域（不强制自动切换 `ONEAGENT_HOME`，但允许用户通过 `--home <workspace>` 让两者一致） <!-- id: 6 -->
- [x] 实现目录结构初始化：`ONEAGENT_HOME/.oneagent/{config,data,logs,tmp}`（最小权限） <!-- id: 7 -->

## 3.5 Workspace（Project）与工具作用域
- [x] 明确定义 workspace：新建会话时可选择/复用 workspace；不启用 workspace 时默认只对话不改文件 <!-- id: 26 -->
- [x] 将文件类工具的默认可写范围对齐到 workspace（workspace 外允许读取任意绝对路径，但默认不允许写/改/删；越界写/改/删返回清晰错误） <!-- id: 27 -->
- [x] 重新定义 `BASH_ROOT_DIR`：默认对齐到当前会话的 workspace（并保留 env/flag 覆盖能力） <!-- id: 28 -->
- [x] 提供统一的“path 规范化 + scope(glob) 匹配”实现，并复用到文件工具/plan/subagent（含 symlink 与 `..` 越界处理与一致错误信息） <!-- id: 29 -->

## 4. 存储层（Settings SQLite + 文件存储；不支持 Postgres）
- [x] 定义 Settings 的最小 schema（仅覆盖 Settings 需要的数据），并实现迁移机制 <!-- id: 9 -->
- [x] Settings 默认落 SQLite：`ONEAGENT_HOME/.oneagent/settings.db`（自动创建/迁移） <!-- id: 10 -->
- [x] 会话/消息/trace/LLM 调用 payload 等其它状态使用文件存储：按 `session_id` 分目录/分文件，落 `ONEAGENT_HOME/.oneagent/data/` 与 `ONEAGENT_HOME/.oneagent/logs/`；LLM 完整 request/response（含 messages）写入 log 文件，trace 仅保存摘要与指针 <!-- id: 11 -->
- [x] 日志按日期目录分层并支持 retention 清理：默认保留 30 天（可配置），启动时 best-effort 清理超期目录；`doctor` 输出日志目录与保留策略 <!-- id: 30 -->
- [x] 增加最小集成测试：Settings SQLite 读写 + 文件存储 round-trip + health/doctor 可诊断 <!-- id: 12 -->

## 5. 认证模式（本地访问令牌；无登录体系）
- [x] 增加 `AUTH_MODE` 配置（token/none；兼容 password 作为别名）并在中间件层实现分支 <!-- id: 13 -->
- [x] 实现 Token Auth：启动时自动生成不过期 token 并持久化到 `ONEAGENT_HOME/.oneagent/config/auth_token`；校验 `Authorization: Bearer <token>` <!-- id: 14 -->
- [x] 前端适配：以“token”作为访问凭证（可复用现有 token 存储逻辑），登录页明确提示“仅建议局域网/公网风险大” <!-- id: 15 -->
- [x] 兼容处理：Keycloak/OAuth 相关接口与前端路由标记为废弃并逐步移除（文档与代码都需对齐） <!-- id: 16 -->

## 6. Doctor 自检与可选依赖
- [x] 实现 `oneagent doctor`：输出版本、profile、home 路径、端口、存储可用性、关键二进制可用性 <!-- id: 17 -->
- [x] 对缺失依赖给出平台化安装建议（macOS: brew；Linux: apt/yum） <!-- id: 18 -->

## 7. 构建与发布（替代 Docker 作为默认交付）
- [x] 增加本地构建脚本（Makefile/justfile）统一 `build-frontend` + `build-backend` + embed 流程 <!-- id: 19 -->
- [x] 增加 release pipeline（本地脚本 + Makefile 入口），产出 macOS/Linux 二进制与 checksum <!-- id: 20 -->
- [x] 更新 README 与 docs：默认运行方式改为本地工具，并将 Docker 标记为可选方案 <!-- id: 21 -->

## 8. 端到端验证（TDD）
- [x] E2E：在无 Docker 情况下启动并完成对话（Token Auth；mock provider） <!-- id: 22 -->
- [x] E2E：缺失可选依赖（如 rg）时，doctor 与工具调用表现符合预期（提示安装，并允许降级为 `grep -R`） <!-- id: 23 -->
- [x] E2E：默认 LAN 监听 + 登录页包含“仅建议局域网/公网风险大”提示（通过嵌入静态资源校验） <!-- id: 24 -->
- [x] E2E：Settings 页面可配置 LLM Provider API Key 与搜索 API Key，重启后仍生效，且接口不会泄露明文 token <!-- id: 25 -->
