## ADDED Requirements

### Requirement: Task attempt 必须固化并展示 tool permissions 快照
系统必须 (MUST) 在每个 task attempt 启动时固化一份 effective tool permissions 快照（例如 policy hash + 解析后的工具集合/关键约束摘要），并将其与该 attempt 关联以便审计与复现。

系统必须 (MUST) 使用该快照作为该 attempt 的权限上限：运行中不得 (MUST NOT) 因用户后续修改策略而“悄然放开”新的权限；若需要提升/变更权限，用户应通过 resume 创建新的 attempt。

#### Scenario: 运行中策略变更不影响当前 attempt
- **GIVEN** 任务 attempt A 启动时的 policy 快照拒绝 `bash`
- **WHEN** 用户在 attempt A 运行过程中将策略更新为允许 `bash`
- **THEN** attempt A 仍不得执行 `bash`
- **AND** 系统提示需要通过 resume 创建新的 attempt 才能应用新策略

