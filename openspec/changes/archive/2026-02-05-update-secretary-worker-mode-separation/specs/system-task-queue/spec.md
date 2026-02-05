# system-task-queue Spec Delta

## ADDED Requirements

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
