## ADDED Requirements
### Requirement: Workspace-first onboarding（优先引导选择 workspace）
系统必须 (MUST) 在用户首次进入 UI 或新建会话时，优先引导用户选择一个本地目录作为 workspace，以便立即使用文件/搜索/命令等工具能力。

系统仍必须 (MUST) 支持用户跳过 workspace，进入“仅对话”模式（保持 `workspace` 能力的可选性）。

#### Scenario: 首次进入且未设置 workspace 时展示引导
- **GIVEN** 用户首次进入聊天页面且当前会话未设置 workspace
- **WHEN** UI 渲染空态（尚未开始对话）
- **THEN** UI 明确提示“选择 workspace 后可直接使用文件/命令工具”
- **THEN** UI 提供“一键选择文件夹”的入口（例如 Browse）
- **THEN** UI 提供“跳过（仅对话）”入口

#### Scenario: 选择 workspace 后立即可开始工作
- **GIVEN** 用户处于 onboarding 引导状态
- **WHEN** 用户选择一个有效目录作为 workspace
- **THEN** UI 将该路径保存为当前会话 workspace
- **THEN** 文件/搜索/命令工具的默认作用域变为该 workspace
- **THEN** 输入框获得焦点，用户可立即发送消息开始工作

#### Scenario: 记住最近 workspace 以减少重复操作
- **GIVEN** 用户曾在该浏览器/客户端上选择过 workspace
- **WHEN** 用户再次进入聊天页面且当前会话未设置 workspace
- **THEN** UI 默认填充最近一次 workspace 并提示用户可一键确认或修改

