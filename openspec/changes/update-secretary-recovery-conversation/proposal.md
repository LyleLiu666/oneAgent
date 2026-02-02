# Proposal: Secretary-guided task recovery conversation

## Why
当前秘书模式对“失败/需要处理”的任务，虽然有卡片与 artifacts 入口，但用户仍需要自己判断：
- 失败原因是什么（尤其是 observer reason/next_steps/questions）
- 需要我回复什么才能继续
- 我回复后任务会不会自动继续、继续到哪里

这违背了我们对“秘书”的期望：**把不危险的排障与推进交给秘书**，用户只需在关键点做决策/补充信息。

## What Changes
- 在秘书模式下，当任务 latest attempt 进入“需要处理”（failed/limit_exceeded/timed_out/interrupted 等终态）时：
  - 秘书应在**对话区**以低噪声方式转达“发生了什么 + 下一步 + 需要用户确认的问题（如果有）”，并附可追溯引用（findings/trace/diff）。
  - 用户可在**对话区直接回复**，系统将该回复作为 `review_notes` 触发 `POST /api/tasks/:id/resume` 继续推进。
- 当同一时间存在多个“需要处理”的事项时，秘书应采用人类助理的汇报方式：
  - “我这里有 N 个事情，接下来一个个请示”，默认从第 1 个开始
  - 每次只推进一个事项，完成/暂缓后再进入下一个（best-effort）
- “排障”行为升级为**优先就地打开 trace**（若存在），只有在无 trace 时才跳转完全模式 `/tasks`。
- 所有动作必须可追溯（聊天里要留痕：哪一个 task/attempt、用什么用户回复触发了 resume）。

## Impact / Non-goals
- 非目标：让秘书在 recovery 阶段执行工具（tool calling）。恢复推进仍由 Task Queue worker 承担。
- 非目标：引入新的远程存储/跨机能力；遵循 local-first、path-only artifacts。
- 兼容性：默认基于现有 `tasks.resume(review_notes)`；如需新增专用 API 仅作为 best-effort 增强。
  - 同时，worker 的输出应尽量先到秘书，再由秘书转达给用户；直通用户的细节入口应折叠，并且用户的操作需要同步给秘书（可追溯）。
