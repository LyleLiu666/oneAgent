## ADDED Requirements

### Requirement: SOP 治理工作台（待治理资产入口）
系统必须 (MUST) 提供一个专门的 SOP 治理工作台页面，集中展示并治理 SOP suggestions（作为待治理资产）。

#### Scenario: 默认展示 proposed/parked 并按得分排序
- **WHEN** 用户打开治理工作台
- **THEN** UI 必须加载 SOP suggestions（至少包含 `proposed` 与可选 `parked`）
- **AND** UI 默认按 `total_score` 降序排序（分值越高越靠前）

#### Scenario: 工作台提供治理动作
- **GIVEN** 某 suggestion 处于 `proposed` 或 `parked`
- **THEN** UI 必须提供：approve（materialize skill）、reject、park、archive、merge、edit、similar 等治理入口（可分组展示）

