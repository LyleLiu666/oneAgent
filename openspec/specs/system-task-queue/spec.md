# system-task-queue Specification

## Purpose
TBD - created by archiving change add-autonomous-task-queue. Update Purpose after archive.
## Requirements
### Requirement: 后台任务（Task）可独立于前端连接持续运行
系统必须 (MUST) 支持创建一个后台任务（Task），并在后台持续推进该任务执行；任务执行不得 (MUST NOT) 依赖浏览器连接或 SSE 是否保持。

任务必须 (MUST) 维护可持久化的“按 attempt（尝试）”状态机；每个 attempt 必须包含唯一标识（例如 `attempt_id` 或 `run_id`）以及状态：
`queued` → `running` → (`succeeded` | `failed` | `canceled` | `timed_out` | `interrupted`)。

系统必须 (MUST) 保留历史 attempts（包含其产物引用），并允许任务在失败后通过“resume”创建新的 attempt 继续推进（见下述要求）。

#### Scenario: 断开浏览器后任务仍继续
- **GIVEN** 用户在某个 workspace 上创建了一个耗时任务，任务进入 `running`
- **WHEN** 用户关闭浏览器或刷新页面导致 SSE 断开
- **THEN** 任务仍在后台继续运行（best-effort）
- **THEN** 用户稍后重新打开 UI 查询任务状态时可看到最新进度与最终结果

#### Scenario: 服务重启导致 running 任务中断但可恢复
- **GIVEN** 任务 attempt 处于 `running`
- **WHEN** oneAgent 服务重启
- **THEN** 该 attempt 被标记为 `interrupted`
- **THEN** 系统不自动创建新的 attempt（等待用户显式 resume）
- **THEN** 该任务可通过 resume 启动新的 attempt 继续推进

### Requirement: Task 以 workspace 为隔离边界
系统必须 (MUST) 将 Task 绑定到一个 workspace（目录路径），并将该 workspace 作为文件工具/命令工具的默认作用域与写入边界。

#### Scenario: Workbench 可按 workspace 聚合任务
- **GIVEN** workspace A 与 workspace B 都存在任务
- **WHEN** 客户端请求任务列表并按 workspace 分组展示
- **THEN** 系统返回的任务数据必须包含 `workspace`
- **AND** 客户端可以在 UI 中筛选/聚合某个 workspace 的任务队列

### Requirement: 每个 workspace 内任务 FIFO 串行执行
系统必须 (MUST) 为同一个 workspace 维护一个 FIFO 队列，并保证同一时间最多只有 1 个 `running` 任务（避免并发修改同一代码库）。

#### Scenario: 同一 workspace 的两个任务按顺序执行
- **GIVEN** 在同一 workspace 上依次创建任务 A 与任务 B
- **WHEN** 任务 A 进入 `running`
- **THEN** 任务 B 保持 `queued`
- **WHEN** 任务 A 结束并进入终态
- **THEN** 任务 B 才能进入 `running`

### Requirement: 不同 workspace 默认并行（不施加固定并行度上限）
系统必须 (MUST) 允许不同 workspace 的任务并行执行；系统不应 (SHOULD NOT) 默认对“并行 workspace 数量”施加固定上限（仅受机器资源约束）。

#### Scenario: 两个 workspace 的任务可以同时 running
- **GIVEN** workspace A 与 workspace B 互不相同
- **WHEN** 用户分别在 A 与 B 上创建任务并开始执行
- **THEN** 系统允许两者同时进入 `running`（best-effort；若资源不足可退化为排队但不得强制串行）

### Requirement: Task 产出可交付且可追溯的产物（summary/findings/trace）
系统必须 (MUST) 为每个完成的任务提供“人类式短总结 + 可追溯引用”：
- `summary`：面向用户的短总结与下一步建议
- `findings_path`：详细交付件路径（包含流水账/变更文件/关键结论）
- `trace_log_path`：完整执行痕迹日志路径（jsonl）

#### Scenario: 成功任务包含产物引用
- **GIVEN** 一个任务执行成功并进入 `succeeded`
- **WHEN** 用户查询任务详情
- **THEN** 响应包含 `summary`
- **THEN** 响应包含 `findings_path` 且该文件存在
- **THEN** 响应包含 `trace_log_path` 且该文件存在

### Requirement: 任务成功判定由 Outcome Observer 基于用户预期完成度决定
系统必须 (MUST) 使用一个独立于主/子 Agent 对话上下文的 Outcome Observer 来判定任务是否 `succeeded`：
- Observer 输入至少包含：任务原始描述（用户预期）、workspace 根目录、以及任务产生的产物引用（例如 findings/trace）。
- Observer 必须 (MUST) 输出 `pass/fail` 与可操作原因，并尽可能引用证据（例如相关文件路径/关键变更）。
- Observer 不得 (MUST NOT) 仅以 “plan/todo 是否完整” 作为成功判定依据（因为 todo 可能不全或不存在）。
- Observer 必须 (MUST) 为只读验收：不得执行命令（例如 `go test`），仅可通过读取文件/列目录/搜索等只读方式获取证据。

#### Scenario: 无 plan 仍可由 Observer 判定任务成功
- **GIVEN** 任务未创建 PLAN.md 或 todo 不完整
- **WHEN** 任务执行结束并进入“待判定”阶段
- **THEN** Observer 仍能基于用户预期与交付产物判定 `succeeded/failed`

#### Scenario: plan 未覆盖全部工作但仍可成功
- **GIVEN** 任务存在 PLAN.md，但 todo 仅覆盖部分工作项
- **WHEN** 任务执行结束并进入“待判定”阶段
- **THEN** Observer 不得因为“存在未列出的 todo”而直接判失败
- **THEN** Observer 以用户预期是否满足为准给出判定与原因

#### Scenario: 通过测试报告等文件证据完成只读判定
- **GIVEN** 任务需要 `go test` 等命令验收
- **WHEN** 主/子 agent 在执行阶段生成一个可读的测试报告文件并将其作为任务产物引用
- **THEN** Outcome Observer 仅通过读取该测试报告与相关文件进行判定（不执行命令）

### Requirement: 任务失败后支持断点接续（Resume）
系统必须 (MUST) 支持对失败/中断类终态的任务进行断点接续：当任务的最近一次 attempt 处于 `failed` / `timed_out` / `interrupted` 时，系统必须 (MUST) 允许用户发起一次 resume，并创建一个新的 attempt 继续推进任务。

resume 的新 attempt 必须 (MUST) 继承原任务的关键上下文（至少包含：原始任务描述、workspace、以及上一次 attempt 的 summary + findings/trace 引用），以最大化复用已完成工作并避免从零开始。

系统必须 (MUST) 保留被 resume 的历史 attempt（其 events/产物引用不可丢失），并在新 attempt 的 events 中记录“由哪个 attempt resume 而来”的关联信息。

#### Scenario: failed 任务 resume 后创建新的 attempt
- **GIVEN** 任务最近一次 attempt 状态为 `failed`
- **WHEN** 用户对该任务发起 resume
- **THEN** 系统创建一个新的 attempt 并进入 `queued`（随后可进入 `running`）
- **THEN** 任务历史 attempts 仍可被查询与回溯

### Requirement: Task 可取消（Cancel）
系统必须 (MUST) 支持用户取消一个任务。

#### Scenario: 取消 running 任务
- **GIVEN** 任务处于 `running`
- **WHEN** 用户请求取消该任务
- **THEN** 系统停止该任务的进一步执行（best-effort）
- **THEN** 任务最终进入 `canceled`（或等价终态）并保留已产生的 events 与产物引用（若有）

### Requirement: Task 支持运行限制（Limits）
系统必须 (MUST) 支持为任务设置可选的运行限制，至少包括：`max_runtime_seconds` 与 `max_steps`，并在超限时将任务标记为失败终态（例如 `timed_out`）。

系统必须 (MUST) 允许用户不提供 limits 创建任务；在未显式设置 limits 的情况下，系统不应 (SHOULD NOT) 默认施加“较小的”运行上限（以支持数小时任务）。

#### Scenario: 未设置 limits 仍允许创建并运行
- **WHEN** 用户创建任务时不提供 `max_runtime_seconds/max_steps`
- **THEN** 系统接受该任务并允许其进入 `running`（best-effort）

#### Scenario: 超时后任务进入 timed_out
- **GIVEN** 任务设置 `max_runtime_seconds=10` 且任务需要更久才能完成
- **WHEN** 任务运行超过 10 秒
- **THEN** 系统停止该任务并将其标记为 `timed_out`

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

### Requirement: Task attempts MUST snapshot effective tool policy
系统必须 (MUST) 在 task attempt 启动时固化一份“effective policy snapshot”，运行期间不得漂移。

#### Scenario: Policy change does not affect running attempt
- **GIVEN** 某 task attempt 已启动并固化 policy snapshot
- **WHEN** 管理员在运行中修改 principal 的 policy
- **THEN** 该 attempt 仍使用启动时的 snapshot
- **AND** 新 policy 仅在新 attempt（resume）中生效

