## ADDED Requirements

### Requirement: Secretary mode MUST surface low-noise task queue hints (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下，以低噪声方式提示 Task Queue 的关键运行状态（best-effort），避免用户在“只聊天”时遗漏后台执行进度。

至少包括（best-effort）：
- 活跃任务数量提示（`GET /api/tasks` 中 `attempt.status in {queued,running}` 的任务数）

#### Scenario: Task hint is visible in secretary mode when active tasks > 0
- **GIVEN** `GET /api/tasks` 返回至少 1 个活跃任务（`queued` 或 `running`）
- **AND** 用户处于秘书模式（best-effort）
- **WHEN** 用户停留在对话主界面
- **THEN** 页面展示一个低噪声 Task badge（best-effort）

#### Scenario: User can enter full mode from the task hint
- **GIVEN** 用户处于秘书模式且 Task badge 可见（best-effort）
- **WHEN** 用户点击该 badge
- **THEN** 系统切换到完全模式（best-effort）
- **AND** 跳转到任务工作台页面（例如 `/tasks`，best-effort）

