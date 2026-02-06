# Change: Refactor secretary canonical session id to be deterministic

## Why
当前 secretary（SU）采用 Settings 中的 `secretary_session_id` 作为 canonical session id，但该值是随机 UUID（见 `backend/internal/runtime/runtime.go`）。

这会导致：
- on-disk 会话目录名不可读、不可预测，排障/运维/手工清理成本高
- 业务语义上“与秘书的对话不分 session（除非用户清空上下文）”与“随机 session_id 目录”不一致

## What Changes
- canonical secretary SU 的 `session_id` 改为**可预测、可复现、文件系统安全**的 deterministic id（按 `principal_id` 派生；local 默认 `secretary`）
- 保持现有语义不变：
  - 清空上下文只删除 SU，不删除 SW（SW 继续保留）
  - SW 仍由 SU 派生（`<su_session_id>-sw`）
- best-effort 迁移：将历史 UUID canonical session（SU/SW）迁移到 deterministic id，尽量不丢消息与 cursor 状态

## Impact
- Affected specs:
  - `openspec/specs/system-secretary-orchestration/spec.md`
- Affected code (expected):
  - `backend/internal/runtime/runtime.go`（ResolveSecretarySessionID + migration）
  - `backend/internal/handler/secretary.go`（无需变更 API；但需要验证 reset 语义保持）
  - `backend/internal/secretary/orchestrator.go`（SW 派生规则保持）
  - `backend/internal/sessionstore/*`（如需复用/增强迁移能力）
- Tests:
  - 新增/更新 Go tests 覆盖 deterministic id 与迁移路径（避免未来维护困难）

