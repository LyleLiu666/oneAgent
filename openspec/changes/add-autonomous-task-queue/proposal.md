# Change: Background task runner + per-workspace queue (agentic, hours-long)

## Why
oneAgent 的产品定位需要从“对话 + 工具调用”升级为“可持续数小时、可交付工业级结果的类人 agent”：用户发起一个复杂任务后无需一直盯着，系统可以持续推进、记录留痕，并在完成后产出可审计的交付物与总结。

在该目标实现后，用户自然会希望进一步扩展到：多个 workspace 并行、每个 workspace 内任务 FIFO 排队（人类只需定期回来验收成果）。

## What Changes
- 新增一个后台任务运行与排队能力：用户可以在指定 workspace 上创建 Task（任务），系统按队列执行并持久化状态。
- Task 运行不依赖前端连接：浏览器关闭/刷新不影响任务推进；用户可随时回来查看任务状态、产物与 trace。
- 每个 Task 必须产生可交付与可追溯的产物（summary + findings_path + trace_log_path），以支持部门级 SOP/审计/复盘。
- 每个 workspace 单 worker 串行执行（FIFO）；不同 workspace 默认不设置“固定并行度上限”（仅受机器资源约束）。
- Task 的 `succeeded/failed` 由一个“更强的观察者（Observer）”基于用户预期进行判定（不要求 todo/plan 完整）。
- Observer 默认只读：不执行 `go test` 等命令；如需测试，由主/子 agent 生成测试报告文件供 Observer 判定。
- 失败/中断任务支持断点接续：保留历史 attempts 与产物引用，可一键 resume 继续推进。

## Impact
- Affected specs:
  - `system-subagent-orchestration`（复用 subagent 的隔离执行 + findings/trace 交付模式）
  - `system-plan-validation`（复用 TDD/observer 验收作为质量闸门）
  - `data-storage`（新增 task records 与 events 的落盘规则）
  - `workspace`（补齐“一个用户可管理多个 workspace 并对其排队任务”的语义）
  - **New**: `system-task-queue`（本次新增能力规格）
- Affected code (expected):
  - Backend: 新增 task queue/store/runner + API（create/list/get/cancel/stream events）
  - Frontend: 新增任务面板/队列视图 + 状态/产物展示
