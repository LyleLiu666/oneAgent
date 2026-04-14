## ADDED Requirements
### Requirement: Chat UI MUST provide a low-noise execution-permission chooser
系统必须 (MUST) 在聊天 UI 中提供低噪声的执行权限入口，让用户无需跳转到 JSON 治理页，也能理解和切换当前执行模式（best-effort）。

至少包括（best-effort）：
- 显示当前模式（只读查看 / 沙箱开发 / 本机执行 / 自定义）
- 提供 simple mode 切换入口
- 提供高风险命令审批模式（自动 / 手动）的简单入口
- 提供“高级设置”跳转到完整工具权限页

#### Scenario: User can see and open the current execution-permission mode from chat
- **GIVEN** 用户位于聊天页（full mode 或 secretary mode；best-effort）
- **WHEN** 页面完成渲染
- **THEN** UI 展示当前执行权限状态（best-effort）
- **AND** 用户可以打开 simple mode chooser（best-effort）

#### Scenario: Advanced policy editing remains available as an escape hatch
- **GIVEN** 用户在聊天页打开执行权限 chooser（best-effort）
- **WHEN** 用户需要更细粒度的策略能力
- **THEN** UI 提供“高级设置 / 自定义策略”入口（best-effort）
- **AND** 该入口跳转到完整工具权限页面（best-effort）

#### Scenario: Advanced settings does not route non-admin users into a dead end
- **GIVEN** 当前用户没有访问完整工具权限治理页的权限（best-effort）
- **WHEN** 用户打开执行权限 chooser（best-effort）
- **THEN** UI 隐藏“高级设置”入口或显示明确不可用说明（best-effort）
- **AND** 不把用户直接送到一个必然失败的治理页面（best-effort）

### Requirement: Secretary mode MUST suggest permission presets instead of sending users to JSON on common blocked flows
系统必须 (MUST) 在秘书模式下，对“权限不足”或“明显需要写/改/跑”的场景，以低噪声方式推荐可理解的权限预设（best-effort），而不是直接把用户推去编辑 JSON。

系统不得 (MUST NOT) 静默提权；用户必须显式选择是否切换。

#### Scenario: Secretary recommends sandbox coding when a task needs write-and-run capability
- **GIVEN** 用户处于秘书模式（best-effort）
- **AND** 当前请求明显需要写文件、改代码或跑测试（best-effort）
- **WHEN** 系统判断当前权限不足以完成该请求（best-effort）
- **THEN** UI 展示一个低噪声权限建议卡片（best-effort）
- **AND** 卡片优先推荐 `沙箱开发`（best-effort）
- **AND** 卡片明确说明切换只影响后续执行，不会让秘书本身直接获得写权限（best-effort）

#### Scenario: Permission-denied flow offers actionable presets instead of only a governance link
- **GIVEN** 某次工具调用因权限策略被拒绝（best-effort）
- **WHEN** UI 在聊天页呈现该状态（best-effort）
- **THEN** 用户可以直接看到并选择相关 simple mode（best-effort）
- **AND** UI 不得只提供“去工具权限页面改 JSON”这一种路径

#### Scenario: Declining the preset keeps the task blocked
- **GIVEN** 系统已在聊天页或秘书模式中展示权限建议（best-effort）
- **WHEN** 用户拒绝切换或关闭该建议（best-effort）
- **THEN** 系统保持原权限配置不变
- **AND** 不继续执行需要更高权限的动作（best-effort）

#### Scenario: UI explains that running tasks keep their current permission snapshot
- **GIVEN** 用户在聊天页或秘书模式中完成了一次 simple mode 切换（best-effort）
- **WHEN** 当前存在仍在运行中的任务（best-effort）
- **THEN** UI 以低噪声方式提示“正在运行的任务保持原权限；新任务或后续重试会使用新配置”（best-effort）
