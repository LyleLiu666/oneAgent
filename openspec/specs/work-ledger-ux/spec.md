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

#### Scenario: SOP proposed count 在 Sidebar 可见
- **GIVEN** `GET /api/ledger/status/today` 返回 `sop_proposed_count > 0`
- **WHEN** 用户打开任意页面（Chat/Tasks/Governance 等）
- **THEN** Sidebar 必须在相关导航项上展示 SOP 数量 badge（例如 `Governance` 或 `Ledger`）

