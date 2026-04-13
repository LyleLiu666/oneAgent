## ADDED Requirements
### Requirement: oneAgent MUST support attaching an optional external formal memory store without replacing Secretary Memory
系统必须 (MUST) 支持在保留本地 Secretary Memory 的前提下，附加一个外部 formal memory store 供 `memorySdk` 使用，但不得把它与现有 `memorydb` 混为同一条主存储链路。

当 external formal memory 启用时，系统必须 (MUST) 保持以下边界：
- `memorydb` 继续承担秘书 SU/SW 的本地 append-only 共享记忆
- `memorySdk` 仅承担 recall / remember / forget / consolidation 这一类正式记忆能力
- 不得通过 `DATABASE_URL` 改写 oneAgent 主存储模型

#### Scenario: 未配置 external formal memory 时仍保持本地默认存储
- **GIVEN** 用户未配置 `MEMORYSDK_POSTGRES_DSN`
- **WHEN** 系统启动 oneAgent
- **THEN** oneAgent 仍按本地默认存储模型启动
- **AND** Settings SQLite、Secretary Memory SQLite、文件存储行为不变

#### Scenario: 配置 external formal memory 时不会替换本地 Secretary Memory
- **GIVEN** 用户配置了 `MEMORYSDK_POSTGRES_DSN`
- **WHEN** 系统启动并初始化 `memorySdk`
- **THEN** 系统连接 external formal memory store 并执行其初始化
- **AND** 本地 `memorydb` 仍继续作为秘书共享记忆存在
- **AND** 系统不得把该 DSN 视为 oneAgent 主存储的 `DATABASE_URL`
