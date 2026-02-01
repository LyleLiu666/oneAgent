## ADDED Requirements

### Requirement: Secretary mode MUST surface low-noise recovery actions for failed tasks (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下，以低噪声方式提示“需要处理”的失败任务，并提供可恢复动作（best-effort），避免用户在“只聊天”心智下错过失败与下一步。

至少包括（best-effort）：
- 失败任务的提示（基于 Task Queue 中终态且非 `succeeded` 的 attempt）
- 一键继续（`POST /api/tasks/:id/resume`）
- 一键进入完全模式排障（例如跳转任务工作台）

#### Scenario: Failed task is surfaced in secretary mode
- **GIVEN** `GET /api/tasks` 返回至少 1 个任务，其 latest attempt 处于失败终态（非 `queued/running` 且非 `succeeded`）
- **WHEN** 用户处于秘书模式并停留在对话主界面
- **THEN** 页面展示该任务的低噪声“需要处理”提示（best-effort）

#### Scenario: User can resume a failed task from secretary mode
- **GIVEN** 失败任务提示已展示（best-effort）
- **WHEN** 用户点击“继续”
- **THEN** 系统调用 `POST /api/tasks/:id/resume` 并将该任务重新入队（best-effort）

#### Scenario: User can enter full mode troubleshooting from secretary mode
- **GIVEN** 失败任务提示已展示（best-effort）
- **WHEN** 用户点击“排障”
- **THEN** 系统切换到完全模式（best-effort）
- **AND** 跳转到任务工作台页面（例如 `/tasks`，best-effort）

