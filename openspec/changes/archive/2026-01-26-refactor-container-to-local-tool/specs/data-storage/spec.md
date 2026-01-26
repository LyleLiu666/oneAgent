# 规范: 本地存储与数据库 (Local Storage)

## ADDED Requirements

### Requirement: local 模式默认具备持久化存储（文件 + Settings SQLite）
系统必须 (MUST) 在 `local` 运行画像下提供默认的持久化存储方案，使用户在未配置任何外部数据库的情况下也能保存会话与配置数据。

系统必须 (MUST) 将持久化拆分为两类：
- **Settings（配置类）**：使用 SQLite 文件 `ONEAGENT_HOME/.oneagent/settings.db`
- **其它状态（会话/消息/trace 等）**：使用文件存储，落在 `ONEAGENT_HOME/.oneagent/data/` 与 `ONEAGENT_HOME/.oneagent/logs/`

#### Scenario: 重启后会话仍可被查询到
- **GIVEN** 用户在 `local` profile 下创建了会话并产生了至少一条消息
- **WHEN** 用户重启 oneAgent
- **THEN** 会话仍可被查询到（来自文件存储，而非数据库）

### Requirement: 会话数据按 session_id 分目录/分文件（降低并发冲突）
系统必须 (MUST) 将会话相关的持久化数据按 `session_id` 进行文件隔离（分目录/分文件），以降低并发写冲突并提升可维护性。

推荐的最小目录结构（示意）：
- `ONEAGENT_HOME/.oneagent/data/sessions/<session_id>/session.json`
- `ONEAGENT_HOME/.oneagent/data/sessions/<session_id>/messages.jsonl`（append-only）

#### Scenario: 两个会话并发写不冲突
- **GIVEN** 会话 A 的 `session_id=a1`，会话 B 的 `session_id=b1`
- **WHEN** 系统分别为 A/B 写入会话数据
- **THEN** A 的写入仅发生在 `.../sessions/a1/` 下
- **THEN** B 的写入仅发生在 `.../sessions/b1/` 下

### Requirement: LLM 调用完整 payload 写入日志文件（替代入库）
系统必须 (MUST) 将每次 LLM 调用的完整 request/response（含 messages）写入日志文件（位于 `ONEAGENT_HOME/.oneagent/logs/` 下），以便成本分析与回溯排障。

系统必须 (MUST) 在 trace 中记录缓存/成本等指标摘要，并保存该次调用对应日志文件的路径指针（trace 只保留“摘要 + 指针”，不回灌大体量 payload）。

系统不得 (MUST NOT) 依赖数据库持久化 LLM 调用的完整 payload（本地模式不再使用 LLMCall 入库作为主路径）。

#### Scenario: trace 仅保存摘要与指针
- **GIVEN** 一次 LLM 调用产生了较大的 messages payload
- **WHEN** 系统持久化本次调用的 trace
- **THEN** trace 中仅包含必要指标摘要（如缓存 key/hash、tokens 等）与日志路径指针
- **THEN** 完整 request/response 位于 `ONEAGENT_HOME/.oneagent/logs/` 下的日志文件中

### Requirement: 日志按日期分层并支持保留策略（retention）
系统必须 (MUST) 将日志按日期目录分层（例如 `.../logs/<type>/YYYY-MM-DD/...`），并支持可配置的日志保留策略以避免磁盘无限增长。

系统必须 (MUST) 在启动时执行 best-effort 的日志清理：删除超过保留天数的日志目录（默认保留 30 天；允许配置覆盖）。

#### Scenario: 启动时清理过期日志
- **GIVEN** 日志目录中存在早于保留天数的日期目录
- **WHEN** oneAgent 启动
- **THEN** 系统清理超期日志目录
- **THEN** 保留期内的日志目录不受影响

### Requirement: 不支持 Postgres / DATABASE_URL
系统必须 (MUST) 不支持 `DATABASE_URL` 指向 Postgres 的路径。

#### Scenario: 设置 DATABASE_URL 时给出明确错误
- **WHEN** 用户设置了 `DATABASE_URL` 并启动 oneAgent
- **THEN** 系统拒绝启动或明确提示“不支持 Postgres/DATABASE_URL”（不得静默连接或静默忽略）

### Requirement: Settings 中配置 token 可持久化且不泄露明文
系统必须 (MUST) 保留 Settings 中对敏感 token 的配置与持久化能力（例如 LLM Provider API Key、搜索 API Key），并确保接口响应不会返回明文 token。

#### Scenario: Provider API Key 仅以 has_api_key 形式暴露
- **WHEN** 用户在 Settings 中创建/更新一个 LLM Provider 并提交 `api_key`
- **THEN** 系统在 Settings SQLite 中持久化该 `api_key`
- **THEN** 通过列表接口读取 provider 时仅返回 `has_api_key=true`（或等价字段），且不返回明文 `api_key`

#### Scenario: 搜索 API Key 可配置且不返回明文
- **WHEN** 用户通过 Settings 设置搜索 API Key（例如 `bocha_api_key`）
- **THEN** 系统在 Settings SQLite 中持久化该 key
- **THEN** 通过 settings 查询接口仅返回 `has_*` 布尔值（或等价字段），且不返回明文 key

### Requirement: 自动迁移与健康检查
系统必须 (MUST) 在启动时对 Settings SQLite 执行必要的迁移（或等价机制），并提供健康检查能力用于诊断。

#### Scenario: 首次启动自动建表
- **WHEN** Settings SQLite 为空（首次启动）
- **THEN** 系统自动创建/迁移必要表结构，且服务可正常处理 Settings 读写

#### Scenario: 健康检查反映存储可用性
- **WHEN** Settings SQLite 不可用（例如文件不可写）
- **THEN** `/health` 或 `doctor` 能反映 degraded 状态并包含原因
- **WHEN** 文件存储目录不可写（例如 `ONEAGENT_HOME/.oneagent/data/` 不可写）
- **THEN** `/health` 或 `doctor` 能反映 degraded 状态并包含原因
