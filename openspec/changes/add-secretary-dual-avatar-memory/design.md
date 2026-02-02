# Design: 永久单会话的秘书双分身 + 共享 Memory（pull-sync）

## Overview
本变更把“秘书”明确建模为：
- **单机永久在线话（single session）**：秘书与用户/系统之间只有一个会话，不再创建/切换会话
- **双分身（双通道）**：SU 面向用户、SW 面向 worker
- **共享 memory（append-only）**：用时间维度可查询的方式保存 worklog/findings，并在 SU/SW 响应前做触发式 pull-sync（last 10 + 省略计数）

关键原则：**尽量不限制行为，而是记录证据**；秘书默认不做写/改，执行交给 worker。

---

## Architecture

### Coordinator（非 LLM 组件）
新增/扩展一个协调器（Coordinator），负责：
1) 路由：用户输入 → SU；worker 事件/问题 → SW
2) Memory：append/query；维护游标；提供 pull-sync 结果
3) Context：SU/SW 各自 80k 阈值的压缩触发（复用现有会话压缩能力）

### 永久单会话
以 `principal_id` 为 key，系统维护唯一的 secretary session：
- 客户端调用 secretary API 时可省略 session_id
- 服务端返回 canonical session_id，但客户端不应创建多个
- 重启后仍可恢复（来自持久化）

---

## Shared Memory（SQLite）

### Data model（建议）
`memory_entries`（append-only）
- `seq INTEGER PRIMARY KEY AUTOINCREMENT`
- `id TEXT UNIQUE`
- `principal_id TEXT NOT NULL`
- `created_at_ms INTEGER NOT NULL`
- `writer TEXT NOT NULL`（SU/SW）
- `type TEXT NOT NULL`（worklog/findings/context_summary）
- `workspace TEXT`
- `title TEXT`
- `content TEXT`
- `links_json TEXT`
- `tags_json TEXT`

索引：
- `idx_memory_entries_principal_created_at (principal_id, created_at_ms)`
- `idx_memory_entries_principal_writer_seq (principal_id, writer, seq)`

`memory_cursors`
- `principal_id TEXT NOT NULL`
- `channel TEXT NOT NULL`（SU/SW：谁在同步）
- `peer_writer TEXT NOT NULL`（SU/SW：同步来源）
- `last_synced_seq INTEGER NOT NULL`

### Query by time
支持 `since_ms/until_ms` 查询（默认 desc + limit）。

---

## Pull-sync（触发式、模拟人类）

### Trigger points（按你的决定）
- SU：准备回复用户前触发
- SW：准备派工/恢复/回复 worker 前触发
- **不要求**在“点击查看进度/交付”等 UI 动作中触发

### Policy
- 只同步“尚未同步”的增量（通过 `last_synced_seq` 去重）
- 只注入最后 10 条（按 seq 取 last 10）
- 若有省略，必须提示：`（还有 N 条较早同步先略过，需要的话我可以按时间再翻。）`
- 注入载荷通过 user prompt，带稳定前缀与定位信息：
  - `[MEMORY_SYNC from=SW type=findings seq=... id=...]`

---

## Context compression（固定 80k）
SU/SW 各自维护上下文窗口，超过 80k 时触发压缩：
- 输出稳定标识的摘要
- 摘要必须包含 timeline + findings
- 压缩失败安全降级（占位提示 + 保留 tail）

---

## UX notes（仅记录要点）
- Secretary 模式应强化“永远只有一个对话”的心智：不展示会话列表/新建会话入口（best-effort）
- Worker UI（展示工具调用/过程噪声）与 Secretary UI 应隔离（可作为后续 change 深化）

