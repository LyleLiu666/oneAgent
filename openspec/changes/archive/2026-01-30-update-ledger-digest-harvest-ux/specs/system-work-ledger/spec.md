## ADDED Requirements

### Requirement: Digest MUST persist a structured representation (in addition to markdown)
系统必须 (MUST) 在生成 Digest 时，除 `markdown` 外还产出结构化 Digest（best-effort），用于 UI 的筛选/聚类/批处理动作：
- 至少包含条目列表（items），每条指向一个 receipt（receipt_id）并含可展示的摘要字段（best-effort）
- 结构化 Digest 必须可被读取而不触发 refresh（best-effort）

#### Scenario: Developer can fetch a structured digest for a day
- **GIVEN** 某天存在 Digest（best-effort）
- **WHEN** 客户端请求该天的结构化 Digest
- **THEN** 返回包含 `items[]` 的 JSON（best-effort）
- **AND** 不会隐式触发 digest refresh（best-effort）

### Requirement: Digest MUST provide failure clustering for harvest mode (best-effort)
系统必须 (MUST) 在结构化 Digest 中提供失败聚类（best-effort），用于用户快速定位“同类问题批量处理”：
- clusters 至少可按 `error_code` / `failure_reason` / `tool_name` 等维度聚合（best-effort）
- cluster 条目包含 count 与 receipt_ids 指针（best-effort）

#### Scenario: Failures are grouped into clusters
- **GIVEN** 当天存在多条 failed receipts，且部分失败原因相同（best-effort）
- **WHEN** 系统生成结构化 Digest
- **THEN** 返回的 `clusters[]` 至少包含一个 cluster，`count>=2`（best-effort）

### Requirement: The system MUST support batch follow-up creation from receipts
系统必须 (MUST) 支持用户从一组 receipts 创建 follow-up（批处理下一轮）：
- 输入是 `receipt_ids[]` + 用户补充指令（可选）（best-effort）
- 输出是一个已入队的 task（或等价执行单元），并保留这次 follow-up 的证据（best-effort）

#### Scenario: User creates a follow-up task from selected receipts
- **GIVEN** 用户选中若干 receipt_ids（>=2 best-effort）
- **WHEN** 用户提交 batch follow-up
- **THEN** 系统创建并入队一个新 task（best-effort）
- **AND** 该 task 的上下文包含所选 receipts 的证据指针（best-effort）
