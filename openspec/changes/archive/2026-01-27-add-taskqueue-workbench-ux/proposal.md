## Why

当前 TaskQueue 已具备后台运行/按 workspace FIFO 串行/跨 workspace 并行的基础能力，但缺少“工作台式”的可视化与治理入口，导致用户无法高效地：
- 同时管理多个 workspace 的任务进展
- 快速定位失败点并断点续跑（resume）
- 观察队列与并行策略是否生效

此外，为未来在单 workspace 内引入更高并行度，需要先明确并落地 L2（OCC 条件写入）契约，避免“基于旧版本探索→写入到新版本”的灾难。

本变更采用：
- **L0**：单 workspace 严格串行（默认）
- **L2（预留）**：条件写入（OCC），写入时要求文件仍处于“读取时的版本”

## What Changes
- 新增 TaskQueue “Workbench” UI：按 workspace 聚合展示任务列表、队列状态、批量操作（cancel/resume）。
- 新增/补齐后端字段与接口契约（如需要）：确保 task/attempt/events 能按 workspace 查询与过滤。
- 定义 L2 OCC “条件写入”契约：工具写入支持 `preconditions`（例如 `expected_sha256` / `expected_mtime`），不满足则拒绝并提示 rebase/retry。

## Non-Goals (v1)
- 不实现 L3 worktree-based isolation/merge
- 不实现跨机器的分布式调度
- 不实现通知/日报（另一个 change）

