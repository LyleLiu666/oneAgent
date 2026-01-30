## ADDED Requirements

### Requirement: Task Workbench MUST have a meaningful empty state in the detail pane
系统必须 (MUST) 在 Task Workbench 的“未选中任务”状态下提供有引导性的空状态，而不是空白死区；空状态至少包含：
- 提示用户选择一个任务或创建新任务
- 指向高频动作的入口（例如聚焦到“新建任务”输入框）

#### Scenario: No selected task shows guided empty state
- **GIVEN** 用户打开 Task Workbench 且当前未选中任何 task
- **WHEN** 详情面板渲染
- **THEN** 详情面板展示空状态引导文案与可执行入口（best-effort）

### Requirement: Task Workbench workspace selector MUST support folder choosing
系统必须 (MUST) 提供比“手输绝对路径”更友好的 workspace 选择方式（例如 folder chooser + 最近使用列表/补全）。

#### Scenario: User chooses workspace via folder picker
- **GIVEN** 用户在 Task Workbench 选择 workspace
- **WHEN** 用户触发“选择文件夹”（或等价入口）
- **THEN** 系统返回并填充标准化的 workspace 路径（best-effort）

### Requirement: Task Workbench MUST not display raw internal error strings
系统必须 (MUST) 在 Task Workbench 中避免展示后端内部错误串；应使用 `system-error-surface` 定义的安全错误披露。

#### Scenario: Create task error is user-safe
- **GIVEN** 用户在 Task Workbench 入队任务失败
- **WHEN** UI 展示错误
- **THEN** 展示安全错误文案与 `request_id`（best-effort）
- **AND** 不直接展示 `err.Error()` 原文

