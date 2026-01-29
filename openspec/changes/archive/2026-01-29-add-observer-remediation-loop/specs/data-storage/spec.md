## MODIFIED Requirements

### Requirement: Task records 与 events 可持久化（本地文件存储）
系统必须 (MUST) 将 Task 的记录与进度事件持久化落盘，以保证：
- 服务重启后仍可查询历史任务与其产物引用
- 用户可在任务运行期间或结束后回溯关键步骤（留痕）

系统必须 (MUST) 使用 append-only 的事件文件（例如 `events.jsonl`）记录任务进度，并使用一个结构化文件（例如 `task.json`）记录任务元数据与终态结果。

#### Scenario: task.json 包含 Outcome Observer 的判定结果与下一步方案
- **GIVEN** 一个任务 attempt 进入终态（succeeded/failed/timed_out/canceled/limit_exceeded/interrupted）
- **WHEN** 系统持久化任务记录到 `task.json`
- **THEN** 该记录包含终态、产物引用（findings/trace），以及 Outcome Observer 的 `pass/fail` 与原因（若已判定）
- **AND** 当 `pass=false` 时，该记录包含 Observer 的 `next_steps`（best-effort；用于后续自动/手动 resume）

