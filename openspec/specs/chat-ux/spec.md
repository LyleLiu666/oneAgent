# chat-ux Specification

## Purpose
TBD - created by archiving change add-secretary-mode-chat. Update Purpose after archive.
## Requirements
### Requirement: Chat UI MUST provide “Secretary Mode” (low-noise)
系统必须 (MUST) 在 Chat UI 中提供一种“秘书模式”以降低默认信息噪声：当用户只想与助手对话时，界面仅保留对话所需的最小要素，其余低频/高级内容默认折叠但可恢复。

秘书模式至少满足：
- 仍可正常发送消息并接收回复
- 默认隐藏低频/高级区域（例如历史侧栏、模型/工具选择、trace/工具细节、任务面板等，best-effort）
- 提供可发现的一键入口切换回完整模式
- 支持通过独立路由直接进入秘书模式（例如 `/secretary`，best-effort），且完整模式仍可从全局菜单进入（best-effort）

#### Scenario: Secretary mode remains usable for chat
- **GIVEN** 用户已进入 Chat 的秘书模式
- **WHEN** 用户发送一条消息
- **THEN** 系统返回助手回复（best-effort）
- **AND** 发送流程不依赖任何高级区域处于展开状态

#### Scenario: User can switch between secretary mode and full mode
- **GIVEN** 用户在秘书模式中
- **WHEN** 用户选择切换到完整模式
- **THEN** 之前被折叠的低频/高级区域变为可见（best-effort）
- **WHEN** 用户再次切换回秘书模式
- **THEN** 低频/高级区域再次折叠（best-effort）

#### Scenario: Secretary mode choice persists across reload
- **GIVEN** 用户已开启秘书模式
- **WHEN** 用户刷新页面或重新打开应用
- **THEN** Chat 仍以秘书模式呈现（best-effort）

#### Scenario: Secretary mode can be entered via dedicated route
- **GIVEN** 系统提供秘书模式独立路由（例如 `/secretary`）
- **WHEN** 用户访问该路由
- **THEN** 页面以秘书模式呈现（best-effort）
- **AND** 用户仍能通过全局菜单进入完整模式（best-effort）

