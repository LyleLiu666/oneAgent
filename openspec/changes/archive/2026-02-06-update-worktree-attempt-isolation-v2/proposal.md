# Change: Harden worktree-based attempt isolation for reliability and rollback

## Why
已有 worktree 模式定义，但在复杂场景（并发、Windows 文件占用、清理失败、孤儿目录）仍可能出现执行漂移和可回退性不足。要提升“可放心交活”的上限，需要把 isolation 细节做成可验证契约。

## What Changes
- 强化 attempt worktree 隔离契约：
  - 启动时记录稳定的 `base_commit_sha` 与 `worktree_root`
  - 运行中禁止越界写入（保持 workspace/write boundary）
- 增加 worktree 生命周期治理：
  - 终态清理策略（立即清理/保留调试）
  - 清理失败可追溯并可重试
  - orphan worktree 周期回收
- 明确 non-git workspace 的失败/降级策略（禁止 silent fallback）

## Impact
- Affected specs:
  - `system-task-queue`
  - `workspace`
- Affected code:
  - attempt 启动流程
  - worktree 生命周期与清理器
  - task artifacts/receipt 记录字段
