# Change: Add git worktree isolation for task attempts

## Why
当前 oneAgent 的默认执行方式是在 workspace root 直接修改文件。对于真实项目，这会带来几个结构性问题：
- attempt 失败/中断后容易留下“半成品状态”，影响下一次执行与用户手工操作
- 并发/多任务的风险很高（即使有 FIFO，用户也可能在 attempt 运行时自行修改）
- 缺少天然的“可审查/可回滚边界”：交付与主线混在一起，diff 审查与回退成本更高

vibe-kanban 的经验是：与其限制 agent 权限，不如让它在 **隔离区（git worktree）** 里大胆干活，然后把“审查/合并”作为进入主线的硬门槛。

## What Changes
- 增加一种可选的 attempt 执行模式：`worktree`（仅对 git workspace 生效）
  - 每次 attempt 在独立的 git worktree 目录执行（隔离文件写入与依赖产物）
  - worktree 基于 attempt 开始时的 base commit/ref 创建，记录 base SHA 作为证据
  - attempt 完成后保留 diff/test/trace 等证据，用户可在 Review 流程中决定是否“合入主线”
- 在 non-git workspace 或创建 worktree 失败时，系统必须给出可操作错误或退化到现有 workspace-root 模式（按配置）
- 增加 worktree 生命周期管理：清理策略、孤儿 worktree 的检测与清理（best-effort）

## Impact
- Affected specs: `workspace`, `system-task-queue`, `system-work-ledger`
- Affected code (expected): `backend/internal/runtime/*`, `backend/internal/taskqueue/*`, `backend/internal/workledger/*`, `backend/internal/tool/*`

