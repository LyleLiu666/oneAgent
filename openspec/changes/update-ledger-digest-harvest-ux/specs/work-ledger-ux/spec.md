## ADDED Requirements

### Requirement: Digest UI MUST support harvest mode (filter + cluster + quick open)
Work Ledger 的 Digest UI 必须 (MUST) 提供面向“收割”的视图能力（best-effort）：
- 按 workspace/status/关键词进行筛选与检索（best-effort）
- 支持按 failure clusters 快速聚合查看（best-effort）
- 每条条目可以一键打开 receipt 详情与关键证据（best-effort）

#### Scenario: User filters digest items and opens receipts quickly
- **GIVEN** Digest 包含多条 items（best-effort）
- **WHEN** 用户选择 `status=failed` 并输入关键词过滤
- **THEN** 列表结果收敛到匹配条目（best-effort）
- **AND** 用户可从条目一键打开对应 receipt（best-effort）

### Requirement: Digest UI MUST provide batch follow-up action
Digest UI 必须 (MUST) 支持用户多选条目并批量发起 follow-up（best-effort）：
- UI 提供多选与计数反馈
- 提供 “Create follow-up task” 主操作，并展示将要发送的概览（best-effort）

#### Scenario: User selects multiple items and creates a follow-up task
- **GIVEN** Digest 视图中存在若干条目（best-effort）
- **WHEN** 用户多选并点击 “Create follow-up task”
- **THEN** UI 发起 batch follow-up 请求并显示成功反馈（best-effort）

