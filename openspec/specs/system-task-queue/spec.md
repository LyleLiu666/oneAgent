# system-task-queue Specification

## Purpose
Defines long-running background task execution with per-workspace FIFO scheduling, durable attempts, resumability, evidence artifacts (summary/findings/trace), and observer-based acceptance.

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
- 当 `pass=false` 时，Observer 必须 (MUST) 同时输出 `next_steps`（下一步可执行方案），用于指导系统继续交付（见下述自动 follow-up 要求）。
- Observer 应该 (SHOULD) 尽量回答/消化执行过程中的“反问/不确定点”，避免把可由证据推断的决策留给用户。
- 仅当确实需要用户偏好或外部信息时，Observer 才可以 (MAY) 输出 `questions_for_user`（并清晰说明为什么需要）。
- Observer 不得 (MUST NOT) 仅以 “plan/todo 是否完整” 作为成功判定依据（因为 todo 可能不全或不存在）。
- Observer 必须 (MUST) 为只读验收：不得执行命令（例如 `go test`），仅可通过读取文件/列目录/搜索等只读方式获取证据。

#### Scenario: Observer fail includes next_steps
- **GIVEN** attempt 的交付产物不足以满足用户预期
- **WHEN** Outcome Observer 判定 `pass=false`
- **THEN** Observer 输出包含 `reason/evidence`
- **AND** 输出包含可执行的 `next_steps`（可以直接写入下一轮 attempt 的 review_notes）

### Requirement: 任务失败后支持断点接续（Resume）
系统必须 (MUST) 支持对终态任务创建新的 attempt 继续推进（包括“失败后的断点接续”以及“成功后的 review follow-up”）：
- 当任务的最近一次 attempt 处于终态（`succeeded`/`failed`/`timed_out`/`interrupted`/`canceled` 等）时，系统必须 (MUST) 允许用户发起一次 resume/follow-up 并创建新的 attempt。
- 新 attempt 必须 (MUST) 继承原任务的关键上下文（至少包含：原始任务描述、workspace、以及上一次 attempt 的 summary + findings/trace 引用）。
- 当用户在 follow-up 时提供 `review_notes`（或等价字段）时，系统必须 (MUST) 将其注入新 attempt 的上下文，并在 events 中记录来源（便于审计）。
- 系统必须 (MUST) 保留被 resume 的历史 attempt（其 events/产物引用不可丢失），并在新 attempt 的 events 中记录“由哪个 attempt resume 而来”的关联信息。

#### Scenario: succeeded 任务也可创建 follow-up attempt
- **GIVEN** 任务最近一次 attempt 状态为 `succeeded`
- **WHEN** 用户对该任务发起 resume/follow-up
- **THEN** 系统创建一个新的 attempt 并进入 `queued`（随后可进入 `running`）
- **THEN** 任务历史 attempts 仍可被查询与回溯

#### Scenario: follow-up 携带 review_notes 注入新 attempt
- **GIVEN** 任务最近一次 attempt 处于终态
- **WHEN** 用户发起 follow-up 并提供 `review_notes="请按 review 修复边界条件，并补充测试"`
- **THEN** 新 attempt 上下文包含该 `review_notes`
- **AND** 新 attempt 的 events 记录该 follow-up 由上一轮 attempt 派生且包含 review_notes 的引用（best-effort）

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

### Requirement: Task limits MUST support cost/token budgets
系统必须 (MUST) 支持为 task 的 `limits` 增加可选的预算字段，用于治理长任务的成本：
- `max_total_tokens`（或等价字段）
- `max_cost_usd`（或等价字段）

#### Scenario: Create task with token budget
- **WHEN** 用户创建 task 并提供 `limits.max_total_tokens`
- **THEN** 系统持久化该预算并在 task 查询中返回

#### Scenario: Budget exceeded stops the attempt
- **GIVEN** 某 attempt 的累计 tokens/cost 即将超过预算
- **WHEN** 系统继续执行下一次 LLM 调用
- **THEN** 系统停止该 attempt 并进入终态
- **AND** summary/trace 中包含“预算耗尽”的可解释原因

### Requirement: Task attempt MUST expose aggregated usage
系统必须 (MUST) 在 task attempt 上暴露累计 usage，以便用户理解“跑了多少、花了多少”（best-effort）：
- `attempt.usage.calls`
- `attempt.usage.prompt_tokens`
- `attempt.usage.completion_tokens`
- `attempt.usage.total_tokens`
- `attempt.usage.cost_usd`（如果可得）

#### Scenario: Get task returns attempt usage
- **GIVEN** 某 attempt 已产生至少一次 LLM 调用
- **WHEN** 用户查询 task
- **THEN** 返回的 attempt 包含 `usage.total_tokens > 0`

#### Scenario: Budget exceeded attempt is resumable
- **GIVEN** 某 attempt 因预算耗尽而进入终态（例如 `status=limit_exceeded`）
- **WHEN** 用户对该 task 执行 resume
- **THEN** 系统创建新的 attempt 并允许继续执行

### Requirement: Task artifacts MUST support a test report artifact
系统必须 (MUST) 支持为 task attempt 产出一个测试报告文件，并将其作为 artifacts 指针暴露（例如 `test_report_path`）。

系统应该 (SHOULD) 在检测到可运行的 tests 时 best-effort 生成该报告；当无法生成时应返回可解释原因（例如缺少依赖/未检测到测试框架）。

#### Scenario: test_report_path is present for Go workspace
- **GIVEN** workspace 是 Go 项目且存在可运行的 tests
- **WHEN** task attempt 执行完成并进入终态
- **THEN** artifacts 包含 `test_report_path`

#### Scenario: test_report_path is omitted with an explanation when tests are unavailable
- **GIVEN** workspace 不包含可运行的 tests（或缺少依赖导致无法执行）
- **WHEN** task attempt 执行完成并进入终态
- **THEN** artifacts MAY 不包含 `test_report_path`
- **AND** attempt summary/trace/receipt 中包含可解释原因（best-effort）

### Requirement: Outcome Observer MUST remain read-only for test evidence
系统必须 (MUST) 保持 Observer 的只读属性：Observer 只能读取 `findings/trace/test_report` 等产物进行判定，不得执行任何测试命令。

#### Scenario: Observer reads test report file only
- **GIVEN** attempt artifacts 中包含 `test_report_path`
- **WHEN** Observer 对该 attempt 做 outcome 判定
- **THEN** Observer 只读取该文件内容用于判定
- **AND** 系统不产生任何“执行测试命令”的 tool call/trace 记录

### Requirement: Mutating attempts MUST have a rollback boundary
系统必须 (MUST) 为任何可能修改用户资产的 task attempt 建立一个“可回退边界”（rollback boundary），以满足“可修改资产必须可回退”的底线。

该边界至少应覆盖 workspace 内的文件系统改动，并满足：
- attempt 开始时生成可恢复的 checkpoint（例如 worktree/base commit、git snapshot、或等价机制）
- attempt 终态后用户可触发 rollback，将 workspace 恢复到 checkpoint 状态（best-effort）
- rollback 不得删除该 attempt 的证据链（trace/findings/diff artifacts 仍需保留用于复盘）

#### Scenario: User rolls back a failed attempt and workspace is restored
- **GIVEN** 一个 attempt 产生了 workspace 文件改动并最终 `failed`
- **WHEN** 用户对该 attempt 触发 rollback
- **THEN** workspace 文件状态被恢复到该 attempt 开始前的 checkpoint（best-effort）
- **AND** 该 attempt 的 artifacts/trace 仍可被查询与打开

### Requirement: Rollback MUST be auditable and idempotent
系统必须 (MUST) 将 rollback 作为一等事件记录到审计证据链，并确保重复触发不会产生额外破坏：
- rollback 事件记录包含 `attempt_id`、checkpoint 引用、操作者（单用户可为 implicit principal）、时间、以及结果（success/fail + reason）
- 对同一 attempt 重复触发 rollback 时，系统应返回“已回退”或等价的幂等结果（best-effort）

#### Scenario: Repeated rollback is idempotent
- **GIVEN** 用户已成功对某 attempt 执行过一次 rollback
- **WHEN** 用户再次对同一 attempt 触发 rollback
- **THEN** 系统返回幂等结果（不再次破坏 workspace）
- **AND** 事件日志包含一次可解释的重复触发记录（best-effort）

### Requirement: Observer fail MUST trigger bounded auto follow-up attempt
当一个 attempt 的执行结束且 Outcome Observer 判定 `pass=false` 时，系统必须 (MUST) 自动创建一个 follow-up attempt 并继续执行，以逼近用户预期交付（best-effort）。

自动 follow-up 必须 (MUST) 满足：
- follow-up attempt 的 `review_notes` 由 Observer 的 `next_steps` 自动填充（可附带简短上下文）
- 系统记录可审计事件（例如 `attempt.queued` data 标注 `source=observer`）
- 自动 follow-up 受 `limits.max_auto_attempts` 约束；达到上限后不得继续自动创建 attempt
- 达到上限后，系统必须 (MUST) 向用户清晰呈现最后一次 Observer 的 `reason + evidence + next_steps`（便于用户手动接管）

#### Scenario: Observer fail auto-enqueues follow-up attempt
- **GIVEN** attempt 执行结束且 Observer 判定 `pass=false`
- **WHEN** 该 task 仍未达到 `limits.max_auto_attempts`
- **THEN** 系统自动创建一个新的 attempt（`status=queued`）
- **AND** 该 attempt 的 `review_notes` 包含 Observer `next_steps`
- **AND** 该 task 被重新入队并继续执行

#### Scenario: Auto follow-up stops after max_auto_attempts
- **GIVEN** 某 task 已自动创建了 `limits.max_auto_attempts` 次 follow-up attempt
- **WHEN** 最新 attempt 再次被 Observer 判定为 `pass=false`
- **THEN** 系统不再自动创建新的 attempt
- **AND** UI/接口返回可解释的失败信息（包含 `next_steps` 供用户接管）

### Requirement: Task attempts MUST produce reviewable change evidence (diff artifacts)
系统必须 (MUST) 为每个 attempt best-effort 产出可审查的“变更证据”，并将其作为 artifacts 指针暴露，以支持 UI diff review 与 outcome 验收（只读）。

当 workspace 是 git repo 时，系统应该 (SHOULD) 优先生成基于 `git diff` 的 patch；当无法生成（非 git / 权限不足 / diff 过大）时，系统必须 (MUST) 至少提供变更文件列表或等价摘要，并在 receipt/trace 中写入可解释原因（best-effort）。

#### Scenario: git workspace attempt 产生 diff patch artifact
- **GIVEN** workspace 是 git repo 且 attempt 产生文件改动
- **WHEN** attempt 进入终态并持久化产物
- **THEN** artifacts 包含 `diff_patch_path`（或等价字段）
- **AND** `diff_patch_path` 指向的文件存在且可读

#### Scenario: 非 git workspace 仍提供变更摘要并解释原因
- **GIVEN** workspace 不是 git repo
- **WHEN** attempt 进入终态并持久化产物
- **THEN** artifacts MAY 不包含 `diff_patch_path`
- **AND** artifacts 包含 `changed_files_path`（或等价摘要）
- **AND** receipt/trace 中包含“无法生成 git diff”的可解释原因（best-effort）

### Requirement: Task attempt MUST run project scripts (best-effort) with evidence
系统必须 (MUST) 在 workspace 存在 `.oneagent/project.json` 时，在 task attempt 生命周期中 best-effort 执行项目脚本，并将输出作为 evidence 纳入产物。

支持的脚本字段（均为可选）：
- `setup_script`：attempt 启动前执行
- `test_script`：attempt 收尾阶段执行（用于生成 test evidence）
- `cleanup_script`：attempt 结束后执行（清理临时文件等）

系统必须 (MUST) 保证脚本执行遵循当前 attempt 的 tool permissions policy（不得绕过策略直接执行）。

当 project config 声明 `copy_files` 时，系统必须 (MUST) 在执行 `setup_script` 之前 best-effort 处理文件复制：
- 复制源必须位于 workspace root 内（不得允许绝对路径或逃逸路径）
- 复制目标为 attempt 的执行目录（默认等于 workspace root；在 worktree/隔离执行场景下可能不同）
- 若任一条目无法复制（源不存在/无权限/目标不可写），系统必须 (MUST) 让 attempt 失败并返回可操作原因（避免后续隐性失败）

#### Scenario: setup_script 执行成功并留下日志
- **GIVEN** workspace 的 `.oneagent/project.json` 包含 `setup_script`
- **WHEN** 系统启动一个新的 task attempt
- **THEN** 系统执行 `setup_script`
- **AND** 将 stdout/stderr 写入 attempt artifacts（或等价可追溯路径）

#### Scenario: setup_script 失败导致 attempt 进入失败并保留证据
- **GIVEN** workspace 的 `.oneagent/project.json` 包含 `setup_script`
- **AND** `setup_script` 退出码非 0
- **WHEN** 系统启动一个新的 task attempt
- **THEN** attempt 进入失败终态（例如 `failed`）
- **AND** attempt summary/receipt 中包含可解释原因（best-effort）
- **AND** 失败时仍保存 stdout/stderr 作为证据

#### Scenario: copy_files 在 setup_script 前被复制到执行目录
- **GIVEN** workspace 的 `.oneagent/project.json` 包含 `copy_files=[".env"]`
- **AND** workspace root 中存在 `.env`
- **WHEN** 系统启动一个新的 task attempt
- **THEN** 系统在执行 `setup_script` 之前将 `.env` 复制到 attempt 执行目录（best-effort）
- **AND** 复制过程遵循 tool permissions policy（不得绕过）

#### Scenario: copy_files 源不存在导致 attempt 失败并返回可操作错误
- **GIVEN** workspace 的 `.oneagent/project.json` 包含 `copy_files=[".env"]`
- **AND** workspace root 中不存在 `.env`
- **WHEN** 系统启动一个新的 task attempt
- **THEN** attempt 进入失败终态（例如 `failed`）
- **AND** summary/receipt 中包含“copy_files 缺失”的可操作原因（指出缺失文件路径）

#### Scenario: test_script 执行并留下测试证据
- **GIVEN** workspace 的 `.oneagent/project.json` 包含 `test_script`
- **WHEN** 一个 task attempt 的主流程执行完成并进入收尾阶段
- **THEN** 系统执行 `test_script`（best-effort）
- **AND** 将 stdout/stderr 写入 attempt artifacts（或等价可追溯路径）
- **AND** 系统在可得时将测试报告引用写入 receipt（例如 `test_report_path`）

#### Scenario: test_script 失败使 attempt 判定为 failed 并可 resume
- **GIVEN** workspace 的 `.oneagent/project.json` 包含 `test_script`
- **AND** `test_script` 退出码非 0
- **WHEN** 系统执行 attempt 的收尾阶段
- **THEN** attempt 进入失败终态（例如 `failed`）
- **AND** summary/receipt 中包含“测试失败”的可解释原因与证据引用（best-effort）
- **AND** 用户仍可通过 resume 创建新 attempt 继续（不应阻断恢复路径）

#### Scenario: cleanup_script 在 attempt 终态后执行且不改变终态
- **GIVEN** workspace 的 `.oneagent/project.json` 包含 `cleanup_script`
- **WHEN** attempt 已进入终态（succeeded/failed/canceled/...）
- **THEN** 系统执行 `cleanup_script`（best-effort）
- **AND** 将 stdout/stderr 写入 attempt artifacts（或等价可追溯路径）
- **AND** `cleanup_script` 的失败不得 (MUST NOT) 覆盖 attempt 的终态（但必须记录原因）

### Requirement: The system MUST support configurable queue governance policies (optional)
系统必须 (MUST) 支持可配置的队列治理策略（best-effort），用于在多 workspace 并行时控制资源消耗，同时保持默认体验不受影响：
- 默认行为保持现状（不同 workspace 可并行且不设固定上限）（best-effort）
- 用户可配置全局并发上限（例如 max_running_tasks / max_running_workspaces）（best-effort）
- 用户可为特定 workspace 覆盖策略（best-effort）

#### Scenario: Global concurrency cap queues additional workspaces
- **GIVEN** 用户配置 `max_running_workspaces=2`（best-effort）
- **WHEN** 三个不同 workspace 的任务同时存在可运行项
- **THEN** 同时最多 2 个 workspace 处于 running，剩余 workspace 任务保持 queued（best-effort）

### Requirement: The system MUST provide workspace queue controls (pause/resume, priority)
系统必须 (MUST) 提供 workspace 队列控制能力（best-effort）：
- pause：暂停该 workspace 的调度（不影响已完成 attempts）
- resume：恢复调度
- priority：支持对 workspace 设定优先级以影响全局调度（best-effort）

#### Scenario: Paused workspace does not start new tasks
- **GIVEN** workspace A 被设置为 paused（best-effort）
- **WHEN** workspace A 的队列中存在 queued tasks
- **THEN** 系统不会为 workspace A 启动新的 running task（best-effort）

### Requirement: The system MUST support scheduled enqueues (best-effort)
系统必须 (MUST) 支持将任务以 schedule 的方式入队（best-effort），用于日程化自动运行（例如每日收割、每周检查等）：
- schedule 至少支持按时间触发（cron/interval 任一即可 best-effort）
- 触发后创建一个普通 task，并进入标准队列策略（best-effort）

#### Scenario: A schedule triggers task enqueue
- **GIVEN** 用户创建一个 schedule（best-effort）
- **WHEN** 触发时间到达
- **THEN** 系统创建一个新 task 并入队（best-effort）

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

### Requirement: TaskQueue Workbench MUST provide a waterfall-style events/log view
系统必须 (MUST) 在 TaskQueue Workbench 中提供一个“瀑布式”的事件/日志视图，用于降低长任务等待期间的不确定性与用户流失风险。

该视图至少应 (SHOULD) 支持：
- `Pretty` / `Raw` 两种展示模式（默认 `Pretty`）
- 按 `attempt_id` 与 `type` 过滤（best-effort）
- 文本搜索（best-effort）
- 展示 `filtered / total` 计数
- 查看单条事件的完整信息（包含完整时间戳、`attempt_id`、以及 `data`）
- Copy filtered（best-effort）

#### Scenario: User filters and inspects event details
- **GIVEN** 某 task 存在多条 events，且部分 events 包含 `data`
- **WHEN** 用户打开该 task 的事件视图并按 `type` 过滤
- **THEN** 列表仅显示匹配的事件（best-effort）
- **WHEN** 用户展开其中一条事件
- **THEN** UI 展示该事件的完整时间戳、`attempt_id` 与 `data`（best-effort）

#### Scenario: Background refresh does not blank the list
- **GIVEN** 用户已打开事件视图并看到事件列表
- **WHEN** 客户端执行后台轮询刷新
- **THEN** UI 不应 (SHOULD NOT) 用“加载中”清空/替换已有列表
- **AND** UI 以轻量方式提示正在刷新（best-effort）

### Requirement: Task attempt artifacts MUST support tail reading for log-like artifacts (best-effort)
系统必须 (MUST) 支持通过 API 获取 attempt 的 log-like artifacts 的“尾部内容”（tail），用于运行中查看最新日志（best-effort）。

系统必须 (MUST) 至少支持对 `trace_log_path`（`trace` artifact）进行 tail 读取；系统应该 (SHOULD) 同样支持 project scripts 的 log artifacts（例如 `setup_script_log` / `test_script_log`）。

#### Scenario: Client requests tail of trace log
- **GIVEN** 某 attempt 已生成并持续写入 `trace_log_path`
- **WHEN** 客户端请求 `GET /api/tasks/:id/attempts/:attempt_id/artifacts/trace?tail=1`
- **THEN** 返回内容包含 trace 文件的最新部分（best-effort）
- **AND** 当文件过大时，响应标记 `truncated=true`（best-effort）

### Requirement: Attempt execution root MUST be isolated when worktree mode is enabled
系统必须 (MUST) 在 worktree 模式启用时，为每个 attempt 创建并使用一个隔离的 git worktree：
- worktree 必须 (MUST) 基于 attempt 启动时的 base commit/ref 创建（记录 base SHA/branch 作为证据）
- worktree 路径必须 (MUST) 被记录到 attempt artifacts（例如 `worktree_root`）以便 UI 打开与审计
- 系统必须 (MUST) 确保 attempt 的文件写入只发生在该 worktree root 内（仍遵循 tool permissions）

#### Scenario: 创建 worktree 并记录 base SHA 与 worktree_root
- **GIVEN** workspace 是 git repo 且启用 worktree mode
- **WHEN** 系统启动一个新的 attempt
- **THEN** 系统创建一个新的 git worktree（best-effort）
- **AND** attempt artifacts 包含 `worktree_root`
- **AND** attempt artifacts 包含 `base_commit_sha`（或等价字段）

### Requirement: Worktree lifecycle MUST be managed with evidence
系统必须 (MUST) 管理 worktree 生命周期，避免“孤儿 worktree”堆积并保证可回溯：
- 系统必须 (MUST) 在 attempt 终态后按策略清理 worktree（默认清理；可配置保留用于调试）
- 当清理失败时，系统必须 (MUST) 记录失败原因并提供可操作提示（例如提示手工清理命令）
- 系统应该 (SHOULD) 提供一个 best-effort 的 orphan worktree cleanup 机制（例如启动时扫描并清理过期 worktrees）

#### Scenario: attempt 完成后按策略清理 worktree
- **GIVEN** attempt 在 worktree mode 下运行并进入终态
- **WHEN** 系统执行 attempt 收尾流程
- **THEN** 系统按策略清理或保留该 worktree
- **AND** receipt/trace 记录该决策与结果（best-effort）

### Requirement: Outcome Observer output MUST be parse-resilient and auto-retried (best-effort)
系统必须 (MUST) 对 Outcome Observer 的结构化输出解析提供 best-effort 的鲁棒性与自愈重试：
- 当 observer 输出不是合法 JSON/XML（例如截断、缺失闭合标签、夹杂多余文本）时，系统应先做 best-effort repair/抽取（例如补全闭合标签、抽取第一个合法块）
- 若仍无法解析，系统必须 (MUST) 以更强约束提示对 observer 发起至少一次重试（best-effort），而不是立刻让 attempt 失败并将工程错误暴露给用户
- 只有当达到重试上限后，才允许将该 attempt 标记为失败（best-effort），并提供 trace/log 指针用于定位

#### Scenario: Truncated <observer_decision> triggers retry and yields a valid decision
- **GIVEN** 某 attempt 已完成并进入 outcome 判定阶段（best-effort）
- **WHEN** observer 首次返回截断的 XML（例如缺失 `</observer_decision>`）（best-effort）
- **THEN** 系统不应立刻失败，而应触发一次受限重试（best-effort）
- **AND** 当重试返回合法结构时，系统正常写入 `attempt.observer` 并继续后续流程（best-effort）
