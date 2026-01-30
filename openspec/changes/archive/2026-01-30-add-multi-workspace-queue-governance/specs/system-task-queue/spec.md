## ADDED Requirements

### Requirement: The system MUST support configurable queue governance policies (optional)
系统必须 (MUST) 支持可配置的队列治理策略（best-effort），用于在多 workspace 并行时控制资源消耗，同时保持默认体验不受影响：
- 默认行为保持现状（不同 workspace 可并行且不设固定上限）（best-effort）
- 用户可配置全局并发上限（例如 max_running_tasks / max_running_workspaces）（best-effort）
- 用户可为特定 workspace 覆盖策略（best-effort）

#### Scenario: Global concurrency cap queues additional workspaces
- **GIVEN** 用户配置 `max_running_workspaces=2`（best-effort）
- **WHEN** 三个不同 workspace 的任务同时存在可运行项
- **THEN** 同时最多 2 个 workspace 处于 running，剩余 workspace 任务保持 queued（best-effort）

### Requirement: The system MUST provide workspace queue controls (pause/resume, priority)
系统必须 (MUST) 提供 workspace 队列控制能力（best-effort）：
- pause：暂停该 workspace 的调度（不影响已完成 attempts）
- resume：恢复调度
- priority：支持对 workspace 设定优先级以影响全局调度（best-effort）

#### Scenario: Paused workspace does not start new tasks
- **GIVEN** workspace A 被设置为 paused（best-effort）
- **WHEN** workspace A 的队列中存在 queued tasks
- **THEN** 系统不会为 workspace A 启动新的 running task（best-effort）

### Requirement: The system MUST support scheduled enqueues (best-effort)
系统必须 (MUST) 支持将任务以 schedule 的方式入队（best-effort），用于日程化自动运行（例如每日收割、每周检查等）：
- schedule 至少支持按时间触发（cron/interval 任一即可 best-effort）
- 触发后创建一个普通 task，并进入标准队列策略（best-effort）

#### Scenario: A schedule triggers task enqueue
- **GIVEN** 用户创建一个 schedule（best-effort）
- **WHEN** 触发时间到达
- **THEN** 系统创建一个新 task 并入队（best-effort）

