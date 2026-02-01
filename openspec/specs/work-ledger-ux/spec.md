# work-ledger-ux Specification

## Purpose
TBD - created by archiving change add-ledger-status-badges. Update Purpose after archive.
## Requirements
### Requirement: Ledger Daily Status Summary API
The system MUST expose a read-only endpoint `GET /api/ledger/status/today` that summarizes today's Work Ledger status for the current principal.

#### Scenario: Returns status without side effects
- **GIVEN** the user has existing Work Ledger data
- **WHEN** the client requests `GET /api/ledger/status/today`
- **THEN** the response MUST include `day_key`, `digest_exists`, `learning_job_status`, and `sop_proposed_count`
- **AND** the endpoint MUST NOT create or refresh a digest
- **AND** the endpoint MUST NOT start or enqueue a learning job

### Requirement: Ledger Status Badges
The Work Ledger UI MUST surface badges that make today's status visible at a glance.

#### Scenario: Badges indicate new/active items
- **GIVEN** `GET /api/ledger/status/today` reports `digest_exists=true`
- **THEN** the Digest tab MUST show a "ready" badge
- **GIVEN** `sop_proposed_count > 0`
- **THEN** the SOP tab MUST show a count badge
- **GIVEN** `learning_job_status` is `queued` or `running`
- **THEN** the Ledger page MUST show a "learning running" badge or equivalent indicator

### Requirement: SOP 治理工作台（待治理资产入口）
系统必须 (MUST) 提供一个专门的 SOP 治理工作台页面，集中展示并治理 SOP suggestions（作为待治理资产）。

#### Scenario: 默认展示 proposed/parked 并按得分排序
- **WHEN** 用户打开治理工作台
- **THEN** UI 必须加载 SOP suggestions（至少包含 `proposed` 与可选 `parked`）
- **AND** UI 默认按 `total_score` 降序排序（分值越高越靠前）

#### Scenario: 工作台提供治理动作
- **GIVEN** 某 suggestion 处于 `proposed` 或 `parked`
- **THEN** UI 必须提供：approve（materialize skill）、reject、park、archive、merge、edit、similar 等治理入口（可分组展示）

### Requirement: Sidebar 显示 SOP badge（全局可见）
系统必须 (MUST) 在 Sidebar / 全局导航中展示关键的 Work Ledger 状态，以提升“非持续盯屏”体验。

在“完全模式”下，Sidebar 必须可见，因此 badge 必须展示在 Sidebar 导航项上。

在“秘书模式”下，当 Sidebar 被隐藏时，系统应该 (SHOULD) 通过低噪声方式在“当前可见的全局导航位置”展示该 badge（例如 Chat header 的角标），或提供一键进入完全模式查看详情的入口（best-effort）。

#### Scenario: SOP proposed count is visible in Sidebar in full mode
- **GIVEN** `GET /api/ledger/status/today` 返回 `sop_proposed_count > 0`
- **AND** 用户处于完全模式（`ui_mode=full`，best-effort）
- **WHEN** 用户打开任意页面（Chat/Tasks/Governance 等）
- **THEN** Sidebar 必须在相关导航项上展示 SOP 数量 badge（best-effort）

#### Scenario: Secretary mode provides a low-noise badge surface (best-effort)
- **GIVEN** `GET /api/ledger/status/today` 返回 `sop_proposed_count > 0`
- **AND** 用户处于秘书模式（`ui_mode=secretary`）且 Sidebar 被隐藏
- **WHEN** 用户停留在对话主界面（best-effort）
- **THEN** 系统在低噪声位置展示 SOP badge 或提供一键进入完全模式入口（best-effort）

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

