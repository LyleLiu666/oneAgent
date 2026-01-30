## ADDED Requirements

### Requirement: Skill governance UI MUST display human-readable skill titles
系统必须 (MUST) 在技能治理页面中将 skill 标识展示为更易读的形式（例如 Title Case、人类化分词），并在需要时仍可查看原始 `skill_id`（best-effort）。

#### Scenario: Skill list shows readable name while preserving ID
- **GIVEN** 存在 skill `code-review-excellence`
- **WHEN** 用户浏览技能列表
- **THEN** 列表展示一个可读标题（best-effort）
- **AND** 用户仍可在详情中看到原始 `skill_id`

### Requirement: Skill governance editor MUST have safe empty state and clear save affordance
系统必须 (MUST) 在技能编辑器区域提供明确的空状态；当未选中可编辑 skill 时，“保存”按钮必须不可用且视觉上不应误导用户可点击。

#### Scenario: Save is disabled and not misleading before selection
- **GIVEN** 用户打开技能治理页面且未选中任何 skill
- **WHEN** 页面渲染
- **THEN** 编辑器展示“请选择技能”的空状态
- **AND** 保存按钮处于禁用状态且具有清晰的禁用反馈（best-effort）
