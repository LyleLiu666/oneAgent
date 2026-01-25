# 任务列表 (Tasks): 容器运行 → 本地工具形应用

> 说明：本文件用于实现阶段（apply）逐条勾选。每一项都应可验证（含测试/自检/手工步骤）。

## 1. 对齐与基线梳理
- [ ] 明确目标画像与范围（local/dev；server 废弃），并在 proposal/design 中锁定关键决策（LAN 默认、共享密码、SQLite） <!-- id: 1 -->
- [ ] 盘点当前 Docker 镜像内置依赖清单（rg/jq/pandoc/ffmpeg/...）并分类：必须/可选/可移除 <!-- id: 2 -->

## 2. CLI 入口与运行画像
- [ ] 新增 `oneagent` CLI 入口（或扩展现有 `cmd/server`）并提供 `--help`/`--version` <!-- id: 3 -->
- [ ] 实现 `oneagent serve --profile {local,dev}` 与基础参数（port、bind、home）（可选：`server` 作为废弃别名仅输出提示并按 local 处理） <!-- id: 4 -->
- [ ] 实现配置优先级：flags > env > config file > defaults，并补齐文档 <!-- id: 5 -->

## 3. 本地 Home/Data 目录
- [ ] 设计并实现 `ONEAGENT_HOME`（或 `--home`）与默认平台路径解析 <!-- id: 6 -->
- [ ] 实现目录结构初始化（config/data/sandbox/logs/tmp）与权限策略（最小权限） <!-- id: 7 -->
- [ ] 将 `BASH_ROOT_DIR` 默认值改为 `ONEAGENT_HOME/sandbox`（并保留 env 覆盖） <!-- id: 8 -->

## 4. 存储层本地化（SQLite 默认；Postgres 可选兼容）
- [ ] 抽象数据库连接层：支持 `sqlite` 与 `postgres` 两种 driver（保持 GORM） <!-- id: 9 -->
- [ ] local profile 默认落 SQLite（`ONEAGENT_HOME/data/oneagent.db`），并确保自动迁移可用 <!-- id: 10 -->
- [ ] （可选）保持 `DATABASE_URL` 指定 Postgres 的兼容路径（迁移/高级用户），并补充连接失败的可诊断日志 <!-- id: 11 -->
- [ ] 为 SQLite/Postgres 增加最小集成测试（建库、AutoMigrate、健康检查） <!-- id: 12 -->

## 5. 认证模式（共享密码；无登录体系）
- [ ] 增加 `AUTH_MODE` 配置（password/none）并在中间件层实现分支 <!-- id: 13 -->
- [ ] 实现 Password Auth：校验共享密码（推荐沿用 `Authorization: Bearer ...` 以最小化前端改动） <!-- id: 14 -->
- [ ] 前端适配：以“密码”作为访问凭证（可复用现有 token 存储逻辑），不再依赖 OAuth/Keycloak 登录流程 <!-- id: 15 -->
- [ ] 兼容处理：Keycloak/OAuth 相关接口与前端路由标记为废弃并逐步移除（文档与代码都需对齐） <!-- id: 16 -->

## 6. Doctor 自检与可选依赖
- [ ] 实现 `oneagent doctor`：输出版本、profile、home 路径、端口、存储可用性、关键二进制可用性 <!-- id: 17 -->
- [ ] 对缺失依赖给出平台化安装建议（macOS: brew；Linux: apt/yum；Windows: winget/scoop） <!-- id: 18 -->

## 7. 构建与发布（替代 Docker 作为默认交付）
- [ ] 增加本地构建脚本（Makefile/justfile）统一 `build-frontend` + `build-backend` + embed 流程 <!-- id: 19 -->
- [ ] 增加 release pipeline（可选：GoReleaser），产出多平台二进制与 checksum <!-- id: 20 -->
- [ ] 更新 README 与 docs：默认运行方式改为本地工具，并将 Docker 标记为可选方案 <!-- id: 21 -->

## 8. 端到端验证（TDD）
- [ ] E2E：在无 Docker 情况下从零启动（local profile），设置/输入共享密码后完成一次对话 <!-- id: 22 -->
- [ ] E2E：缺失可选依赖（如 rg）时，doctor 与工具调用表现符合预期（降级/提示） <!-- id: 23 -->
- [ ] E2E：在局域网内通过其它设备访问 UI（默认 LAN 监听），且来自公网的访问默认被阻止（或有明确的显式开关） <!-- id: 24 -->
- [ ] E2E：Settings 页面可配置 LLM Provider API Key 与搜索 API Key，重启后仍生效，且接口不会泄露明文 token <!-- id: 25 -->
