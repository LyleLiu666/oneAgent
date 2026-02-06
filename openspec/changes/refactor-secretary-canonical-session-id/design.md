## Context
当前 canonical secretary session id 由 Settings 以随机 UUID 方式生成并持久化（`secretary_session_id`）。
虽然“稳定”（best-effort），但对维护者不友好：目录不可读、定位日志困难，也与“与秘书的对话不分 session”的产品心智不一致。

## Goals
- canonical secretary SU session id 对同一 `principal_id` **可预测、可复现、跨重启稳定**
- 目录名文件系统安全（禁止路径穿越；避免奇怪字符）
- 保持现有行为：Reset 只清 SU，不清 SW
- best-effort 迁移旧数据，不让用户“凭空丢上下文/待确认状态”

## Non-Goals
- 不改变 session store 的总体布局（仍按 `session_id` 分目录）
- 不引入按日期拆分消息文件（可作为后续单独 change）
- 不重构 SU/SW 的职责边界

## Proposed Design
### Deterministic session id scheme
- 对每个 `principal_id` 派生 canonical SU session id：
  - `principal_id` 为空视作 `local`
  - `principal_id=local` 时使用固定 `session_id=secretary`（更贴近产品心智）
  - 其它 principal 使用 `secretary-<safe_slug>-<hash8>`（避免仅 slug 导致碰撞）
- canonical SW session id 仍沿用既有派生：`<su_session_id>-sw`

### Migration strategy (best-effort)
在 `ResolveSecretarySessionID(principal_id)` 首次命中 legacy UUID 时触发迁移：
1. 计算 `desiredSU`（deterministic）
2. 读取 Settings 中旧值 `legacySU`（若不存在则直接写入 `desiredSU` 并返回）
3. 若 `legacySU == desiredSU`：直接返回
4. 若不同：
   - 若 `desiredSU` 不存在且 `legacySU` 存在：迁移 SU 会话（session.json + messages.jsonl）
   - 对 SW：`legacySW = legacySU + "-sw"`，`desiredSW = desiredSU + "-sw"`，按同样规则迁移（若存在）
   - 迁移成功后，将 Settings 值更新为 `desiredSU`
   - 若 `desiredSU` 已存在：避免合并风险，改为直接切换 Settings 到 `desiredSU`，保留 legacy 目录供手动排查（并输出日志提示）

迁移要求：
- 尽量保留 message ids（不要通过 append 重写导致 id 变化）
- session.json 的 `session.id` 必须与目录名一致

### Concurrency
迁移应在服务端做一次性操作，避免并发下重复迁移：
- 在 runtime 内部为 secretary canonical 解析增加 per-principal 的 mutex（best-effort）

