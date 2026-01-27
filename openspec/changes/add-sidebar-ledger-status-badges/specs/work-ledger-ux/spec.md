## ADDED Requirements

### Requirement: Sidebar 显示 SOP badge（全局可见）
系统必须 (MUST) 在 Sidebar / 全局导航中展示关键的 Work Ledger 状态，以提升“非持续盯屏”体验。

#### Scenario: SOP proposed count 在 Sidebar 可见
- **GIVEN** `GET /api/ledger/status/today` 返回 `sop_proposed_count > 0`
- **WHEN** 用户打开任意页面（Chat/Tasks/Governance 等）
- **THEN** Sidebar 必须在相关导航项上展示 SOP 数量 badge（例如 `Governance` 或 `Ledger`）
