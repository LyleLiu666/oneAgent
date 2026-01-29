# Design: Observer remediation loop

## Goals
- **挂机收菜**：Observer 判失败不是“结束”，而是“下一轮可执行输入”的来源。
- **证据链完整**：Observer 输出的理由/证据/下一步方案必须落盘，成为可追溯资产。
- **止损**：必须有明确边界（attempt 次数/预算），避免无限循环与成本失控。
- **默认简单**：UI 默认只展示最关键信息，复杂细节可折叠展开。

## Non-goals
- 不把 Observer 变成可执行 agent：Observer 仍保持只读，不执行任何命令。
- 不在本变更内引入审批/回滚等更大安全框架（另行 change）。

## Data model
ObserverDecision 扩展字段：
- `next_steps`: string（当 `pass=false` 时 MUST 提供；可作为下一轮 attempt 的 `review_notes` 直接注入）
- `questions_for_user`: string[]（可选；仅当确实需要用户偏好/外部信息时使用）

Task limits 扩展字段：
- `max_auto_attempts`: int（自动 follow-up attempt 次数上限；不限制用户手动 resume）

## Control flow
Attempt 执行结束后：
1) 系统调用 Outcome Observer 判定结果（只读）
2) 若 `pass=true`：attempt `succeeded`
3) 若 `pass=false`：
   - attempt 进入 `failed`，记录 ObserverDecision（含 `next_steps`）
   - 若未达到 `max_auto_attempts`，系统自动创建 follow-up attempt（`queued`），并将 `next_steps` 写入 `review_notes`
   - follow-up attempt 自动入队执行（FIFO per workspace）
4) 达到上限后停止自动重试，向用户呈现“失败原因 + 下一步建议”，用户仍可手动 resume

## Guardrails
- 自动 follow-up 仅在 “Observer fail” 时触发；不对 `canceled/timed_out/limit_exceeded` 等终态自动重试。
- `next_steps` 长度需要截断（避免无限膨胀污染上下文）。

