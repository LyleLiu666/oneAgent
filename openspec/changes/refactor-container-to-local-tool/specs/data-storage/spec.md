# 规范: 本地存储与数据库 (Local Storage)

## ADDED Requirements

### Requirement: local 模式默认具备持久化存储
系统必须 (MUST) 在 `local` 运行画像下提供默认的持久化存储方案，使用户在未配置 `DATABASE_URL` 时也能保存会话与配置数据。

#### Scenario: 未设置 DATABASE_URL 时仍可持久化
- **WHEN** 用户在 `local` profile 启动且未设置 `DATABASE_URL`
- **THEN** 系统使用默认本地数据库（例如 SQLite 文件）进行持久化
- **THEN** 重新启动后会话仍可被查询到

### Requirement: 可选的 Postgres 兼容路径（迁移/高级用户）
系统必须 (MUST) 在设置 `DATABASE_URL` 时使用 Postgres（与现有行为兼容），用于迁移或高级用户场景（不再依赖 server profile 语义）。

#### Scenario: DATABASE_URL 指向 Postgres
- **WHEN** 用户设置 `DATABASE_URL=postgres://...` 并启动
- **THEN** 系统连接 Postgres 并完成健康检查与迁移（或给出可诊断错误）

### Requirement: Settings 中配置 token 可持久化且不泄露明文
系统必须 (MUST) 保留 Settings 中对敏感 token 的配置与持久化能力（例如 LLM Provider API Key、搜索 API Key），并确保接口响应不会返回明文 token。

#### Scenario: Provider API Key 仅以 has_api_key 形式暴露
- **WHEN** 用户在 Settings 中创建/更新一个 LLM Provider 并提交 `api_key`
- **THEN** 系统在数据库中持久化该 `api_key`
- **THEN** 通过列表接口读取 provider 时仅返回 `has_api_key=true`（或等价字段），且不返回明文 `api_key`

#### Scenario: 搜索 API Key 可配置且不返回明文
- **WHEN** 用户通过 Settings 设置搜索 API Key（例如 `bocha_api_key`）
- **THEN** 系统在数据库中持久化该 key
- **THEN** 通过 settings 查询接口仅返回 `has_*` 布尔值（或等价字段），且不返回明文 key

### Requirement: 自动迁移与健康检查
系统必须 (MUST) 在启动时执行数据库迁移（AutoMigrate 或等价机制）并提供健康检查能力用于诊断。

#### Scenario: 首次启动自动建表
- **WHEN** 数据库为空（首次启动）
- **THEN** 系统自动创建/迁移必要表结构，且服务可正常处理会话读写

#### Scenario: 健康检查反映存储可用性
- **WHEN** 数据库不可用（例如文件不可写或连接失败）
- **THEN** `/health` 或 `doctor` 能反映 degraded 状态并包含原因
