## ADDED Requirements

### Requirement: Tool permissions UI MUST clearly show the current principal context
系统必须 (MUST) 在工具权限治理页面清晰标识当前正在编辑/查看的 `principal_id` 以及该 principal 的生效策略快照信息（best-effort）。

#### Scenario: Principal context is visible
- **GIVEN** 用户在工具权限页面查看策略
- **WHEN** 页面渲染完成
- **THEN** 页面清晰展示当前 `principal_id` 与 policy snapshot 基本信息（best-effort）

### Requirement: Tool policy editor MUST be usable for real JSON
系统必须 (MUST) 提供一个可用的策略编辑体验：足够高度、等宽字体、格式化/校验入口（best-effort）；避免“迷你 textarea”导致不可用。

#### Scenario: Policy editor supports formatting and comfortable editing
- **GIVEN** 用户编辑 tool policy JSON
- **WHEN** 用户输入/粘贴策略内容
- **THEN** 编辑器区域具备足够高度且可一键格式化（best-effort）

