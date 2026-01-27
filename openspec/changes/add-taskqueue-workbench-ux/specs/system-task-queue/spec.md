## MODIFIED Requirements

### Requirement: Task 以 workspace 为隔离边界
系统必须 (MUST) 将 Task 绑定到一个 workspace（目录路径），并将该 workspace 作为文件工具/命令工具的默认作用域与写入边界。

#### Scenario: Workbench 可按 workspace 聚合任务
- **GIVEN** workspace A 与 workspace B 都存在任务
- **WHEN** 客户端请求任务列表并按 workspace 分组展示
- **THEN** 系统返回的任务数据必须包含 `workspace`
- **AND** 客户端可以在 UI 中筛选/聚合某个 workspace 的任务队列

## ADDED Requirements

### Requirement: TaskQueue Workbench UI
系统必须 (MUST) 提供一个 TaskQueue 工作台，用于在多 workspace 场景下可视化任务队列与运行状态。

#### Scenario: 用户在一个页面管理多个 workspace 的任务
- **GIVEN** 用户有多个 workspace
- **WHEN** 用户打开 TaskQueue Workbench
- **THEN** 用户可以看到每个 workspace 的任务列表（含 `queued/running/terminal`）
- **AND** 用户可以对任务执行 `cancel/resume`
- **AND** 用户可以查看 task attempt history 与 events

### Requirement: 条件写入（OCC Preconditions）
系统必须 (MUST) 支持在文件写入/编辑工具中携带 preconditions，并在不满足时拒绝写入，以避免“基于旧版本探索的修改落到新版本”。

#### Scenario: 文件内容变化导致写入被拒绝
- **GIVEN** 一个 task 在读取 `foo.go` 时记录了 `expected_sha256`
- **WHEN** 另一任务先修改了 `foo.go` 导致 sha 变化
- **THEN** 原 task 尝试写入 `foo.go` 时必须被拒绝
- **AND** 错误信息必须包含“文件已变化”的可操作提示（例如要求重新读取/重试/rebase）

