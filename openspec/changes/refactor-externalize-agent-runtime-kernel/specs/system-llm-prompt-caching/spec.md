## MODIFIED Requirements
### Requirement: prompt_cache_key 必须与会话稳定段绑定并支持 epoch
当 provider 支持 `prompt_cache_key` 时，系统必须 (MUST) 为同一会话在多轮之间复用 cache key，并在“稳定前缀发生结构性变化”（如模型切换、工具协议切换、会话压缩）时支持引入 `epoch` / `prefix version` 以更新 cache key，避免不可解释的 miss 或潜在碰撞。

系统必须 (MUST) 直接复用共享运行内核提供的 cache key helper（best-effort），而不是在宿主层另写一套独立规则；宿主层只负责提供输入（session id / prefix version / model / tool protocol / stable messages / tool schema）。

稳定签名必须 (MUST) 至少覆盖（best-effort）：
- 非 volatile 的 system messages
- 非 volatile 且被标记为 `force_cacheable` 的稳定摘要消息
- JSON tool schema（当 tool protocol 为 JSON 时）

#### Scenario: 改 turn context 不改变 cache key
- **GIVEN** 同一会话、相同 stable prefix、相同工具 schema（best-effort）
- **AND** 只有 volatile 的 turn context 发生变化（best-effort）
- **WHEN** 系统生成 `prompt_cache_key`（best-effort）
- **THEN** cache key 保持不变（best-effort）

#### Scenario: 改 summary 或 tool schema 会改变 cache key
- **GIVEN** 同一会话的 stable summary 被更新，或工具 schema 发生变化（best-effort）
- **WHEN** 系统生成新的 `prompt_cache_key`（best-effort）
- **THEN** cache key 发生变化（best-effort）

### Requirement: 缓存能力矩阵与回退策略
系统必须 (MUST) 维护显式的 provider 缓存能力矩阵，并覆盖系统已集成的所有 provider（不得只在部分 provider 下生效）。

当系统进入“外置共享运行内核”模式时，系统应该 (SHOULD) 直接复用 SDK 的 provider cache 兼容语义（best-effort）；宿主层只负责记录降级事件、provider/model、以及最终是否成功重试。

#### Scenario: 普通 4xx 不应误触发 cache 降级
- **GIVEN** 用户启用了缓存（best-effort）
- **AND** provider 返回的是与 cache 无关的普通 4xx（best-effort）
- **WHEN** 系统处理该错误（best-effort）
- **THEN** 系统不会把该错误误判成 prompt cache 降级（best-effort）
- **AND** 系统不会执行额外的 cache 兼容重试（best-effort）
