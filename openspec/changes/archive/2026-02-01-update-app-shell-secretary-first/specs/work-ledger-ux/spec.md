## MODIFIED Requirements

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

