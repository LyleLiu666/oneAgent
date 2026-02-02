## MODIFIED Requirements

### Requirement: local 模式默认具备持久化存储（文件 + Settings SQLite）
系统必须 (MUST) 在 `local` 运行画像下提供默认的持久化存储方案，使用户在未配置任何外部数据库的情况下也能保存会话、配置数据与秘书记忆（Memory）。

系统必须 (MUST) 将持久化拆分为三类：
- **Settings（配置类）**：使用 SQLite 文件 `ONEAGENT_HOME/.oneagent/settings.db`
- **Secretary Memory（记忆类）**：使用 SQLite 文件 `ONEAGENT_HOME/.oneagent/memory.db`（best-effort；允许实现选择等价路径）
- **其它状态（会话/消息/trace 等）**：使用文件存储，落在 `ONEAGENT_HOME/.oneagent/data/` 与 `ONEAGENT_HOME/.oneagent/logs/`

#### Scenario: 重启后会话与记忆仍可被查询到
- **GIVEN** 用户在 `local` profile 下创建了会话并产生了至少一条消息（best-effort）
- **AND** 系统写入了至少一条 Secretary Memory entry（best-effort）
- **WHEN** 用户重启 oneAgent
- **THEN** 会话仍可被查询到（来自文件存储，而非数据库）
- **AND** Secretary Memory 仍可被查询到（来自 Memory SQLite；best-effort）

## ADDED Requirements

### Requirement: Secretary Memory MUST support time-range queries (best-effort)
系统必须 (MUST) 将 Secretary Memory 以 append-only 的方式持久化，并支持按时间维度进行 best-effort 查询，以满足“记忆具有时效性、需要按时间快速回溯”的使用方式。

查询能力至少包括（best-effort）：
- `since_ms` / `until_ms` 的时间窗过滤
- `limit` 截断（默认按时间倒序）

#### Scenario: Query memory entries by time window (best-effort)
- **GIVEN** 系统已为同一 `principal_id` 写入多条带 `created_at_ms` 的 Memory entries（best-effort）
- **WHEN** 客户端以 `since_ms/until_ms` 发起查询（best-effort）
- **THEN** 系统仅返回落在该时间窗内的 entries（best-effort）
- **AND** 结果按时间倒序返回并遵守 `limit`（best-effort）

