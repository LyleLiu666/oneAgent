# Change: Add diff review loop for task attempts

## Why
在代码/文档类任务中，“Agent 完成一次 attempt”并不等于“交付可合并”。用户通常需要：
- 快速看到做了哪些改动（diff/变更文件）
- 基于改动给出 review comments（而不是重新描述需求）
- 让这些 comments 直接进入下一轮 attempt（follow-up），形成短反馈闭环

如果缺少 diff review 闭环，用户只能在聊天里反复追问“你改了什么/为什么”，并且评论无法结构化沉淀到证据链与后续执行中，导致效率低、风险高、可回溯性差。

## What Changes
- 为每个 task attempt 生成“变更审查证据”（best-effort）：变更文件列表 + diff patch（git workspace 优先）
- 在 UI 中提供最小可用的 Review 入口：查看 diff、测试证据、以及提交 review comments
- 支持从任意终态（包括 `succeeded`）创建 follow-up attempt，将 review comments 与上一轮证据引用注入新 attempt 上下文
- review comments 必须进入 Work Ledger/Receipt 的证据链，便于复盘与追溯

## Impact
- Affected specs: `system-task-queue`, `system-work-ledger`
- Affected code (expected): `backend/internal/taskqueue/*`, `backend/internal/workledger/*`, `backend/internal/server/*`, `frontend/src/views/*`, `frontend/src/components/*`

