## Context
当前 oneAgent 已具备：
- 会话持久化（sessions/messages/trace 日志落盘）
- `plan` 工具 + observer 验收（TDD 质量闸门）
- `subagent` 隔离执行（可配置 max_steps/max_runtime_seconds，产出 findings + trace_log_path）
- chat handler 使用 `context.Background()`，即使前端断连也能继续生成

但仍缺少“像人一样持续推进并交付”的关键产品层能力：
- 明确的“任务（Task）”一等公民（创建/排队/运行/取消/查看产物）
- 任务的可观察进度与留痕（events）
- 按 workspace 串行的调度，避免同一 workspace 内并发写导致混乱
- 多 workspace 并行执行（同 workspace 串行，跨 workspace 尽可能并行）
- “任务是否成功”的强判定标准（不依赖 todo/plan 完整）

## Goals / Non-Goals
- Goals
  - 任务可后台运行数小时（best-effort），前端不需要常驻。
  - 每个 workspace 内任务 FIFO 串行，避免并发修改同一代码库。
  - 任务产出必须可审计：summary + findings_path + trace_log_path（复用 subagent handoff 模式）。
  - 提供取消与状态查询，支持用户定期验收成果。
- Non-Goals（本次不做或可选）
  - 商业化分发/计费（skills/provider marketplace）
  - 任务跨机器分布式执行（先本地单机）
  - 精确恢复到同一 LLM 流程节点的强 SLA 断点续跑（本次先做“基于产物与上下文摘要的 resume”）

## Decisions
- Decision: 任务执行引擎优先复用 `subagent`（作为后台 worker）
  - `subagent` 已具备：隔离上下文、长 runtime、tools、findings/trace 产物、plan.mark_done 记录，最接近“工业级交付”。
  - TaskRunner 只负责调度与状态机，不把复杂推理/执行逻辑再实现一遍。
- Decision: “按 workspace 单 worker 串行 + 跨 workspace 尽可能并行”的调度模型
  - 同一 workspace 只允许一个 Running task。
  - 不同 workspace 默认不设置固定并行度上限（仅受机器资源约束）；如未来需要运维限流，可作为可选配置引入。
- Decision: 任务与事件落盘采用文件存储（遵循 data-storage 约定）
  - Task record: `ONEAGENT_HOME/.oneagent/data/tasks/<task_id>/task.json`
  - Events: `.../events.jsonl`（append-only）
  - 产物：复用 subagent 的 `findings_path/trace_log_path` 并在 task.json 中引用
- Decision: Task 成功判定使用“Outcome Observer”
  - 不要求 plan/todo 完整：Observer 基于“用户任务描述 + workspace 状态 + 产物引用（findings/trace）”判定是否满足用户预期。
  - 输出必须包含可操作的原因与证据引用（例如涉及哪些文件/变更）。
- Decision: Outcome Observer 严格只读（不执行命令）
  - 如需 `go test` 等命令验收，由主/子 agent 在执行阶段运行并生成测试报告文件；Observer 仅基于文件/报告做判定。
- Decision: 失败任务支持断点接续（resume）
  - Task 以 “attempt” 作为执行尝试的边界：每次 attempt 产生独立的 run_id/findings/trace，并保留历史。
  - resume 会启动一个新的 attempt，并以“上一次 attempt 的 summary + findings/trace 引用”作为输入上下文，最大化复用已完成工作与证据。

## API Sketch (non-normative)
- `POST /api/tasks`：创建任务（workspace、title、prompt、model_id、limits）
- `GET /api/tasks?workspace=...`：列表
- `GET /api/tasks/:id`：详情
- `POST /api/tasks/:id/cancel`：取消
- `GET /api/tasks/:id/events`：拉取 events（可 SSE）

## UX Sketch (non-normative)
- Workspace 维度的“任务队列面板”
  - New Task（输入任务描述，选择 workspace + model）
  - 队列列表（queued/running/done/failed/canceled）
  - 详情页：实时 events、findings、trace 入口、变更文件列表、最终总结

## Risks / Trade-offs
- 长时间运行成本与失控风险（token/cost）
  - 用户期望默认不设严格 limits：需要提供取消能力与可选 limits（以及后续可加 cost cap）
- 同一 workspace 的并发写风险
  - 通过 per-workspace 串行 worker 避免
- 服务重启导致内存队列丢失
  - v1：任务记录持久化 + 启动时恢复 queued 状态（running 任务标记为 failed 或可配置为 retry）
- 断点接续的“真实续跑”复杂度
  - v1 以“基于上一次 attempt 的产物与上下文摘要的 resume”实现近似断点续跑（不要求精确恢复到同一 LLM 节点）

## Open Questions
- 服务重启恢复策略：running 任务在重启后应标记为 `interrupted` 并允许一键 resume，还是自动 resume（可选配置）？
