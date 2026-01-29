## ADDED Requirements

### Requirement: SOP governance UI MUST avoid “ghost actions” and provide helpful empty states
系统必须 (MUST) 在 SOP 治理页面对空列表/未选中状态提供友好的空状态，并避免展示无意义或不可用的“幽灵按钮”（例如无数据时的加载更多）。

#### Scenario: Empty SOP list shows guidance and hides irrelevant actions
- **GIVEN** SOP 建议列表为空
- **WHEN** 用户打开 SOP 治理页面
- **THEN** 页面展示引导性空状态（例如说明何时产生建议）
- **AND** 无意义的操作按钮默认不展示或明确禁用（best-effort）

### Requirement: SOP governance copy MUST be user-centered by default
系统必须 (MUST) 在 SOP 治理页面默认文案中避免暴露实现术语（例如“稀缺性/know-how/证据排序”）；可将解释放入可选的帮助入口（例如 tooltip/info，best-effort）。

#### Scenario: Default subtitle is understandable without implementation jargon
- **WHEN** 用户打开 SOP 治理页面
- **THEN** 页面默认副标题使用用户可理解的表述（best-effort）
