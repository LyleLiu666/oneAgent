## ADDED Requirements
### Requirement: 跨平台 workspace 选择（picker 可选，手动输入必备）
系统必须 (MUST) 在所有平台提供“手动输入 workspace 路径并立即校验/规范化”的能力，确保用户不会因为缺少 native folder picker 而无法启用 workspace。

系统应该 (SHOULD) 在 local-tool 场景提供 OS-native 的 workspace folder picker（例如 Windows/macOS）。

#### Scenario: Windows 上可通过 picker 选择 workspace
- **GIVEN** oneAgent 运行在 Windows 且处于 local-tool 场景（UI 与服务同机）
- **WHEN** 用户在 UI 中点击 “Browse/Choose workspace”
- **THEN** 系统打开 folder picker 并返回用户选择的目录
- **THEN** 系统对目录进行规范化并作为 workspace 保存

#### Scenario: picker 不可用时可手动输入并通过校验
- **GIVEN** 系统不支持 folder picker（例如远程 server、或平台限制）
- **WHEN** 用户在 UI 输入一个 workspace 目录路径并保存
- **THEN** 系统对该路径进行规范化与存在性校验
- **THEN** 校验通过后该路径作为 workspace 生效
- **THEN** 校验失败时返回清晰错误原因（例如路径不存在/不可访问）

