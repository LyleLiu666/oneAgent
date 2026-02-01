## ADDED Requirements

### Requirement: Secretary mode MUST surface low-noise deliverable cards for completed task artifacts (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下，以低噪声方式呈现 Task Queue 已完成任务的“可交付产物入口”（best-effort），且交付卡片范围仅覆盖任务产物（Task artifacts），不引入额外的管理系统信息结构。

至少包括（best-effort）：
- 展示最近完成的 task attempt（终态）对应的交付卡片（限量 N 个以控制噪声）
- 每张卡片提供 artifacts 的可点击入口（例如 findings/diff/test_report/trace）
- 用户点击 artifact 后可在秘书模式内预览其内容（best-effort），无需切换到完全模式

#### Scenario: Deliverable cards appear for completed tasks
- **GIVEN** `GET /api/tasks` 返回至少 1 个任务，其 latest attempt 处于终态（非 `queued/running`）
- **AND** 该 attempt 存在至少一个可用 artifact（best-effort）
- **WHEN** 用户处于秘书模式并停留在对话主界面
- **THEN** 页面展示对应的 deliverable card（best-effort）

#### Scenario: User can preview a task artifact from a deliverable card
- **GIVEN** deliverable card 已渲染且包含 findings artifact（best-effort）
- **WHEN** 用户点击 findings 入口
- **THEN** 系统调用 task artifact 预览接口并展示内容（best-effort）
- **AND** 不切换到完全模式（best-effort）

