# system-secretary-orchestration Spec Delta

## MODIFIED Requirements

### Requirement: Secretary MUST use a canonical permanent session per principal (best-effort)
系统必须 (MUST) 为每个 `principal_id` 提供一个**唯一且持久化**的 secretary session（best-effort），用于承载秘书与用户/系统之间的长期对话；该 secretary session 在刷新页面、服务重启后仍保持一致（best-effort）。

系统应该 (SHOULD) 允许客户端在调用 secretary 相关 API 时省略 `session_id`，由服务端根据 `principal_id` 解析并返回 canonical `session_id`（best-effort）。

为了降低维护与排障成本，系统必须 (MUST) 保证 canonical secretary `session_id` 是**可预测/可复现**且**文件系统安全**的（best-effort）：
- 不应 (SHOULD NOT) 为 canonical secretary session 使用随机 UUID 作为 `session_id`（除非为了兼容迁移历史数据）
- 对 `principal_id=local`，系统应该 (SHOULD) 使用固定 `session_id=secretary`（best-effort）
- 对其它 principal，系统必须 (MUST) 以 deterministic 方式由 `principal_id` 派生 `session_id`，并避免不同 `principal_id` 因 slug/sanitize 发生碰撞（best-effort）

当系统发现历史版本已存在 legacy canonical session（例如随机 UUID）时，系统应该 (SHOULD) 进行 best-effort 的迁移，使用户不会“凭空丢失上下文/待确认状态”（best-effort）。

#### Scenario: Secretary session id is stable across reload and restart (best-effort)
- **GIVEN** `principal_id=P` 已产生过一次 secretary 对话（best-effort）
- **WHEN** 用户刷新页面并再次进入秘书模式（best-effort）
- **THEN** 系统解析到同一个 canonical `session_id`（best-effort）
- **WHEN** oneAgent 重启后用户再次进入秘书模式（best-effort）
- **THEN** 系统仍解析到同一个 canonical `session_id`（best-effort）

#### Scenario: Canonical secretary session id is deterministic and discoverable (best-effort)
- **GIVEN** `principal_id=local`（best-effort）
- **WHEN** 系统解析 canonical secretary session（best-effort）
- **THEN** 返回 `session_id=secretary`（best-effort）
- **AND** 对应消息落盘路径可预测：`ONEAGENT_HOME/.oneagent/data/sessions/secretary/messages.jsonl`（best-effort）

