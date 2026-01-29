## ADDED Requirements

### Requirement: Skill governance UI MUST use progressive disclosure
系统必须 (MUST) 在技能治理 UI 中使用渐进式披露，默认仅展示高频入口，并将低频/高级操作折叠起来。

高频入口至少包含：
- 技能列表与选择
- 个人技能的保存/归档（当可用时）

低频/高级内容（例如 duplicates/pin/archive-shadowed、路径/sha 等诊断信息）必须 (MUST) 默认折叠，并提供可发现的展开入口。

#### Scenario: Advanced governance actions are collapsed by default
- **GIVEN** 用户打开技能治理页面
- **WHEN** 页面首次渲染完成
- **THEN** 高级区域默认处于折叠状态
- **AND** 页面仍可完成技能查看/编辑/归档等高频操作

#### Scenario: User can expand advanced section to manage duplicates
- **GIVEN** 页面存在 duplicates 治理能力
- **WHEN** 用户展开高级区域
- **THEN** 用户可以执行 pin/archive-shadowed 等低频治理操作（best-effort）

