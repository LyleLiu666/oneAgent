# Change: Add default task limits

## Why
长时运行的 agentic 任务如果缺乏默认限制，容易出现不可控的资源消耗（时间、步骤、成本），也会让“失败断点接续/队列治理”变得困难。

## What Changes
- 为 task queue 增加 **默认 limits**：
  - 当创建 task 未指定 `limits` 时，系统会填充默认 `max_steps` 与 `max_runtime_seconds`。
  - 允许通过环境变量覆盖默认值与上限（cap），用于部门级治理。
- runner 执行时使用相同的 limits 解析逻辑，确保旧任务/旧数据也能获得一致行为。

## Impact
- Affected specs: `local-runtime`
- Affected code: `backend/internal/taskqueue/*`, `backend/internal/handler/tasks.go`, `backend/internal/server/taskqueue_runner.go`

