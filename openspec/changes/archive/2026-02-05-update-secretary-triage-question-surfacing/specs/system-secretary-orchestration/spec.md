## ADDED Requirements
### Requirement: Triage MUST surface actionable user confirmations when `questions[]` is non-empty (best-effort)
当 triage 结果包含 `questions[]`（非空）时，系统必须 (MUST) 以“可操作”的方式把这些待确认点呈现给用户（best-effort），避免只返回计数或模糊话术导致用户无法继续。

至少满足（best-effort）：
- **逐条可见**：每个 question 的内容必须对用户可见（在对话摘要或可发现的 UI 入口中至少一种）。
- **下一步明确**：必须说明用户如何回复/如何选择才能继续推进（best-effort）。
- **可追溯恢复**：刷新/重进会话后，用户仍可看到当前未解决的 `questions[]`（best-effort；可通过 `GET /api/secretary/state` 等恢复）。
- **避免内部术语**：秘书对用户的描述应避免“派工/worker/workspace”等内部实现词；如必须涉及路径/目录等概念，应使用用户语言解释（best-effort）。

#### Scenario: Triage returns questions and the user can see exactly what to confirm
- **GIVEN** triage 对某次消息归并输出 `questions[]` 且非空（best-effort）
- **WHEN** 系统写入秘书汇报消息或返回 triage 响应（best-effort）
- **THEN** 用户在秘书模式下可以看到每条问题的具体内容（best-effort）
- **AND** 用户可以从汇报中得知下一步如何回复以继续（best-effort）

#### Scenario: Questions remain visible after refresh (best-effort)
- **GIVEN** triage 产生了未解决的 `questions[]`（best-effort）
- **WHEN** 用户刷新页面或重新进入会话（best-effort）
- **THEN** 系统可通过 secretary state 恢复这些 `questions[]` 以便继续确认（best-effort）
