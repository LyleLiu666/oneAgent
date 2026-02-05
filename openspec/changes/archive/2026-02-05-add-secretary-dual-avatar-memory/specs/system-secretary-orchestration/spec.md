## ADDED Requirements

### Requirement: Secretary MUST use a canonical permanent session per principal (best-effort)
系统必须 (MUST) 为每个 `principal_id` 提供一个**唯一且持久化**的 secretary session（best-effort），用于承载秘书与用户/系统之间的长期对话；该 secretary session 在刷新页面、服务重启后仍保持一致（best-effort）。

系统应该 (SHOULD) 允许客户端在调用 secretary 相关 API 时省略 `session_id`，由服务端根据 `principal_id` 解析并返回 canonical `session_id`（best-effort）。

#### Scenario: Secretary session id is stable across reload and restart (best-effort)
- **GIVEN** `principal_id=P` 已产生过一次 secretary 对话（best-effort）
- **WHEN** 用户刷新页面并再次进入秘书模式（best-effort）
- **THEN** 系统解析到同一个 canonical `session_id`（best-effort）
- **WHEN** oneAgent 重启后用户再次进入秘书模式（best-effort）
- **THEN** 系统仍解析到同一个 canonical `session_id`（best-effort）

### Requirement: Secretary orchestration MUST be split into SU/SW channels with distinct responsibilities (best-effort)
系统必须 (MUST) 将秘书编排划分为两个逻辑通道（双分身；best-effort）：
- `Secretary(User)`（SU）：面向用户对话与汇报；默认只读（best-effort）
- `Secretary(Work)`（SW）：面向执行层派工/恢复/答疑；默认只 dispatch，不直接面向用户（best-effort）

系统必须 (MUST) 将用户输入路由给 SU，将 worker 的事件/提问路由给 SW（best-effort）。

#### Scenario: User messages are routed to SU while worker events are routed to SW (best-effort)
- **GIVEN** 同一 `principal_id` 下存在 SU 与 SW 两个通道（best-effort）
- **WHEN** 用户在秘书模式发送一条消息（best-effort）
- **THEN** 系统将该输入交由 SU 处理并生成用户侧回复（best-effort）
- **WHEN** 某 worker 产生“需要秘书介入”的事件或提问（best-effort）
- **THEN** 系统将该输入交由 SW 处理并生成对 worker 的答复或派工（best-effort）

### Requirement: SU/SW MUST share an append-only Memory and perform trigger-based pull-sync (last 10) (best-effort)
系统必须 (MUST) 提供一个 SU/SW 共享的 append-only Memory（best-effort），用于记录：
- 流水账（worklog）
- findings（结论/约束/交付物指针/待办）
- 上下文压缩摘要（context_summary；best-effort）

系统必须 (MUST) 在 SU/SW “准备生成回复”前触发 pull-sync（best-effort）：
- 仅同步对方通道中“尚未同步”的增量（通过 cursor 去重，best-effort）
- **只注入最后 10 条**（按时间/序号排序后取末尾 10 条，best-effort）
- 若存在省略，必须提示省略条数（best-effort）

#### Scenario: SU pull-syncs from SW before replying and reports omitted count (best-effort)
- **GIVEN** SW 写入了 23 条新的 Memory entries，且 SU 尚未同步（best-effort）
- **WHEN** SU 准备对用户生成回复（best-effort）
- **THEN** 系统为 SU 注入来自 SW 的最后 10 条 entries（best-effort）
- **AND** 系统告知“已省略 13 条较早同步”（best-effort）
- **AND** 系统更新 SU→SW 的同步 cursor（best-effort）
