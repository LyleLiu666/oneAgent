# system-secretary-orchestration Spec Delta

## ADDED Requirements

### Requirement: Secretary triage MUST be built on the shared Agent Factory (best-effort)
系统必须 (MUST) 允许 secretary triage（SW planner / SU report）复用共享的 Agent Factory（best-effort），以继承一致的：
- prompt assembly（stable prefix / volatile turn context）
- tool protocol 选择策略（即使 triage 本身不执行工具）
- 留痕（message list + trace pointers）

#### Scenario: Secretary uses the same agent runtime building blocks as worker chat (best-effort)
- **GIVEN** 项目已提供 Agent Factory（best-effort）
- **WHEN** 系统构建 secretary triage planner（SW）与 user-facing secretary（SU）（best-effort）
- **THEN** 其 prompt assembly 与 KV-cache 策略来自同一套基础设施（best-effort）
- **AND** 不会重复实现一套“强制 JSON + 手写 parse”的专用路径（best-effort）

### Requirement: Progress/status questions MUST NOT require brittle keyword routing or extra user clarification (best-effort)
当用户在秘书模式下询问进度/状态/完成情况（例如“任务完成得怎么样”“现在有几个任务在进行”）时，系统必须 (MUST) 直接给出基于系统事实的进度回复（best-effort），而不是反问用户“是哪一个任务/在哪个目录”等无谓澄清，尤其在系统只存在 1 个相关任务时（best-effort）。

系统应该 (SHOULD) 在 triage planner 的上下文中提供 best-effort 的 `tasks_snapshot`（只读快照，易变内容），以减少模型“看不到任务状态而只能反问”的情况。

#### Scenario: Only one task exists; progress question does not trigger clarification (best-effort)
- **GIVEN** 当前 principal 只有 1 个相关任务处于 running/queued/just-finished（best-effort）
- **WHEN** 用户询问“任务完成得怎么样”（best-effort）
- **THEN** 系统返回基于任务状态的进度摘要（best-effort）
- **AND** 不会要求用户先回答“你指的是哪个任务/哪个目录”（best-effort）

### Requirement: Secretary triage MUST express intent via a structured output channel (best-effort)
系统必须 (MUST) 为 secretary triage planner 提供一种结构化方式表达“本轮意图”（best-effort），例如：
- `intent=progress`（只需汇报进度，不派工）
- `intent=dispatch`（生成派工计划并可能产生 questions）
- `intent=clarify`（需要用户确认后再派工）

该 intent 必须通过 tool-call 或宽松 tags 返回（best-effort），以避免在代码层面做 brittle 的 NLP 关键词路由。

#### Scenario: Planner returns progress intent for a progress query (best-effort)
- **GIVEN** 用户询问进度（best-effort）
- **WHEN** triage planner 输出结构化结果（best-effort）
- **THEN** intent=progress（best-effort）
- **AND** SU 根据系统数据生成确定性的进度汇报（best-effort）
