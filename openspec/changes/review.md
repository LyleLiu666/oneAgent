# OpenSpec Changes: Priority & Progress

更新时间：2026-02-06

本文件只记录**当前 active changes** 的优先级与进度快照；历史内容不再维护。
已完成变更请看 `openspec/changes/archive/`，真实进度以 `openspec list` 为准。

## Priority（高 → 低）

### P0（本地工程交付主线：简单/复杂问题都能交付且留痕）
- `add-head2head-benchmark-suite`：head-to-head 基准评测（指标可回归）
- `update-task-deliverable-contract-v1`：交付物契约化（manifest/证据一致）
- `update-worktree-attempt-isolation-v2`：worktree 隔离执行鲁棒化（可回滚边界）
- `update-secretary-autonomy-selfheal-v2`：秘书更少打断用户（先自愈后升级）

### P1（规模化稳定性：可解释、可观测、可恢复）
- `update-queue-governance-scheduling-v2`：多 workspace 调度治理（公平性/可解释/日程化）

### P2（可选：外部触发入口）
- `update-mcp-action-plane-v1`：MCP 从只读扩展到受控动作面（create/resume/cancel）

### P3（更后：渠道/IM 接入，当前阶段不做）
- `add-channel-relay-v1`：渠道中继 v1（入站委托 + 出站通知）

### Done（治理基线）
- `update-openspec-truth-alignment`：OpenSpec 真相对齐（Purpose/状态快照可校验）

## Snapshot（来自 `openspec list`）
- `update-openspec-truth-alignment`：✓ Complete
- `add-head2head-benchmark-suite`：✓ Complete
- `update-task-deliverable-contract-v1`：✓ Complete
- `update-worktree-attempt-isolation-v2`：✓ Complete
- `update-secretary-autonomy-selfheal-v2`：✓ Complete
- `update-queue-governance-scheduling-v2`：0/12 tasks
- `update-mcp-action-plane-v1`：0/12 tasks
- `add-channel-relay-v1`：0/14 tasks
