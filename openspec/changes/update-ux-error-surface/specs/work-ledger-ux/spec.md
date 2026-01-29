## ADDED Requirements

### Requirement: Ledger receipts list MUST be scannable
系统必须 (MUST) 在 Ledger 的回执列表中优先呈现可扫描信息（summary/status/time），对长文本进行摘要/截断，避免“纯文本堆砌”。

#### Scenario: Receipt list uses summary-first layout
- **GIVEN** 存在多条 receipt 且 summary 较长
- **WHEN** 用户查看 receipt 列表
- **THEN** 列表项展示摘要/状态/时间并对长文本截断（best-effort）

### Requirement: Receipt detail view MUST provide incremental context
系统必须 (MUST) 在回执详情中提供相对于列表的增量信息（例如 artifacts 入口、关联 task/attempt 的可追溯引用），避免仅复读列表内容（best-effort）。

#### Scenario: Receipt detail includes artifacts references
- **GIVEN** 用户打开某条 receipt 的详情
- **WHEN** 详情渲染完成
- **THEN** 详情包含可追溯的 artifacts 引用入口（best-effort）
