## MODIFIED Requirements

### Requirement: TaskQueue Workbench UI
系统必须 (MUST) 提供一个 TaskQueue 工作台，用于在多 workspace 场景下可视化任务队列与运行状态。

系统必须 (MUST) 在该工作台 UI 中使用渐进式披露（progressive disclosure）：
- 默认仅展示高频入口：workspace 选择、任务入队、任务列表、任务详情（含状态/摘要）与 cancel/resume
- 将低频/高级内容（例如可选预算、证据路径、policy snapshot、事件列表等）默认折叠，并提供可发现的展开入口

#### Scenario: 用户在一个页面管理多个 workspace 的任务
- **GIVEN** 用户有多个 workspace
- **WHEN** 用户打开 TaskQueue Workbench
- **THEN** 用户可以看到每个 workspace 的任务列表（含 `queued/running/terminal`）
- **AND** 用户可以对任务执行 `cancel/resume`
- **AND** 用户可以查看 task attempt history 与 events

#### Scenario: Advanced sections are collapsed by default
- **GIVEN** 用户打开 TaskQueue Workbench
- **WHEN** 页面首次渲染完成
- **THEN** 高级区域默认处于折叠状态（best-effort）
- **AND** 页面仍可完成任务入队与 cancel/resume 等高频操作

