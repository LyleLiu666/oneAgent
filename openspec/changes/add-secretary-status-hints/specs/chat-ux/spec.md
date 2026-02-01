## ADDED Requirements

### Requirement: Secretary mode MUST surface low-noise status hints (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下，以低噪声方式展示关键状态提示（best-effort），避免用户在“只聊天”时错过可治理/可收割的事项。

至少包括（best-effort）：
- SOP 待治理数量（`GET /api/ledger/status/today.sop_proposed_count`）

#### Scenario: SOP badge is visible in secretary mode when proposed count > 0
- **GIVEN** `GET /api/ledger/status/today` 返回 `sop_proposed_count > 0`
- **AND** 用户处于秘书模式（best-effort）
- **WHEN** 用户停留在对话主界面
- **THEN** 页面展示一个低噪声 SOP badge（best-effort）

#### Scenario: User can enter full mode from the hint
- **GIVEN** 用户处于秘书模式且 SOP badge 可见（best-effort）
- **WHEN** 用户点击该 badge
- **THEN** 系统切换到完全模式（best-effort）
- **AND** 跳转到 SOP 治理页面（例如 `/governance/sop`，best-effort）

