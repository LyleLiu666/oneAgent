## ADDED Requirements

### Requirement: UI 内任务完成通知（无外部 webhook）
系统必须 (MUST) 在 UI 内提供任务状态更新的可见性，帮助用户在不持续盯屏的情况下快速发现“已完成/失败/可续跑”的任务。

#### Scenario: queued/running 进入终态时产生 UI 通知
- **GIVEN** 用户打开 Task Workbench 或 Chat 内 Task Panel
- **WHEN** 某 task 的最新 attempt 状态从 `queued` 或 `running` 变为终态（`succeeded` / `failed` / `timed_out` / `interrupted` / `canceled`）
- **THEN** UI 应显示一条任务更新提示（包含 task 标题或 id、workspace、状态）

#### Scenario: 首次加载不为历史任务产生通知
- **WHEN** 用户首次打开页面并加载任务列表
- **THEN** UI 仅建立“已知状态基线”，不为此前已完成的历史任务生成通知

