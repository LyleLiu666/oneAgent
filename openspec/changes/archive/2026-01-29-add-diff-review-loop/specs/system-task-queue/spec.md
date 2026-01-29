## MODIFIED Requirements

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

## ADDED Requirements

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

