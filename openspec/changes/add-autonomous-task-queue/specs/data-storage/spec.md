## ADDED Requirements
### Requirement: Task records 与 events 可持久化（本地文件存储）
系统必须 (MUST) 将 Task 的记录与进度事件持久化落盘，以保证：
- 服务重启后仍可查询历史任务与其产物引用
- 用户可在任务运行期间或结束后回溯关键步骤（留痕）

系统必须 (MUST) 使用 append-only 的事件文件（例如 `events.jsonl`）记录任务进度，并使用一个结构化文件（例如 `task.json`）记录任务元数据与终态结果。

#### Scenario: 重启后任务仍可被列出
- **GIVEN** 系统中存在至少一个已创建的任务（任意状态）
- **WHEN** oneAgent 服务重启
- **THEN** 用户仍可通过任务列表接口看到该任务（来自文件存储）

#### Scenario: task.json 包含 Outcome Observer 的判定结果
- **GIVEN** 一个任务进入终态（succeeded/failed/timed_out/canceled）
- **WHEN** 系统持久化任务记录到 `task.json`
- **THEN** 该记录包含终态、产物引用（findings/trace），以及 Outcome Observer 的 `pass/fail` 与原因（若已判定）

#### Scenario: task.json 记录 attempts 历史以支持 resume
- **GIVEN** 一个任务经历了多次 attempt（例如失败后 resume）
- **WHEN** 系统持久化任务记录到 `task.json`
- **THEN** `task.json` 包含 attempts 列表，且每个 attempt 记录其状态与产物引用（findings/trace/可选测试报告等）
- **THEN** 历史 attempt 的产物引用不可丢失

#### Scenario: events.jsonl 事件可关联到具体 attempt
- **GIVEN** 一个任务包含多个 attempt
- **WHEN** 系统持续写入任务事件到 `events.jsonl`
- **THEN** 每条事件包含 attempt 标识（例如 `attempt_id`/`run_id`）以便按 attempt 回溯
