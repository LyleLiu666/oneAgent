# Change: Add observer remediation loop (next-step guidance + auto follow-up)

## Why
在 “挂机收菜” 的工作台模式下，一个任务对用户来说是一个目标；对系统来说应该是可持续逼近的交付闭环。

当前流程里 Outcome Observer 只负责判定 `pass/fail`。当判定为 `fail` 时，系统会把 attempt 标记为失败并停止，等待用户手动 `resume`。
这会导致两个问题：
- 用户拿到的是 “fail”，而系统明知 “还差什么” 却不继续交付，违背“类人交付”的定位。
- LLM 在执行过程中常常会出现 “反问/卡住”（需要下一步决策或补充证据）。Observer 如果只输出判定，而不输出下一步方案，则其价值被削弱。

本变更把 Observer 的作用扩展为：**判定 + 给出下一步可执行方案**，并让系统在失败时 **自动创建 follow-up attempt** 继续推进，直到交付或触发止损。

## What Changes
- Outcome Observer 输出结构扩展：
  - 除 `pass/reason/evidence` 外，新增 `next_steps`（下一步方案，供后续 attempt 直接执行）
  - 可选 `questions_for_user`（仅在确实需要用户偏好/外部信息时才输出）
- 当 Observer 判定 `fail` 且仍有自动重试预算时，系统自动创建一个 follow-up attempt：
  - follow-up attempt 的 `review_notes` 由 Observer 的 `next_steps` 自动填充
  - 系统自动入队并继续执行（best-effort）
- 引入 `limits.max_auto_attempts`（默认值可配置）作为止损，避免无限循环；达到上限后停止自动重试并向用户展示“还差什么 + 下一步建议”。

## Impact
- Affected specs: `system-task-queue`, `data-storage`
- Affected code (expected):
  - Backend: `backend/internal/taskqueue/*`（observer schema/prompt + runner auto follow-up + limits）
  - Frontend: Task workbench/queue 中展示 observer 的 `next_steps`（progressive disclosure）

