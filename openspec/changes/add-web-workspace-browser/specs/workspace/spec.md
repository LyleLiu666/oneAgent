## MODIFIED Requirements
### Requirement: 跨平台 workspace 选择（picker 可选，手动输入必备）
系统必须 (MUST) 在所有平台提供“手动输入 workspace 路径并立即校验/规范化”的能力，确保用户不会因为缺少 native folder picker 而无法启用 workspace。

系统应该 (SHOULD) 在 local-tool 场景提供 OS-native 的 workspace folder picker（例如 Windows/macOS）。

当服务端环境不支持 OS-native picker，但服务端文件系统对前端可见时，系统必须 (MUST) 提供网页目录浏览器作为选择辅助，并且该浏览器只能浏览服务端明确允许的目录根范围。

#### Scenario: Windows 上可通过 picker 选择 workspace
- **GIVEN** oneAgent 运行在 Windows 且处于 local-tool 场景（UI 与服务同机）
- **WHEN** 用户在 UI 中点击 “Browse/Choose workspace”
- **THEN** 系统打开 folder picker 并返回用户选择的目录
- **THEN** 系统对目录进行规范化并作为 workspace 保存

#### Scenario: Docker 中可通过网页目录浏览器选择容器内目录
- **GIVEN** oneAgent 运行在 Docker 或 Linux 场景，且原生 picker 不可用
- **AND** 服务端提供了允许浏览的目录根列表
- **WHEN** 用户在 UI 中点击 “选择文件夹”
- **THEN** 系统展示网页目录浏览器
- **AND** 用户可以进入允许根目录下的子目录并选择当前目录
- **AND** 最终选择的目录会被规范化并作为 workspace 保存

#### Scenario: 网页目录浏览器拒绝越界路径
- **GIVEN** 服务端提供网页目录浏览器
- **WHEN** 客户端请求浏览允许根目录之外的路径
- **THEN** 系统拒绝该请求并返回清晰错误

#### Scenario: picker 与网页浏览器都不可用时可手动输入并通过校验
- **GIVEN** 系统不支持 folder picker，且当前服务端也未开放网页目录浏览器
- **WHEN** 用户在 UI 输入一个 workspace 目录路径并保存
- **THEN** 系统对该路径进行规范化与存在性校验
- **THEN** 校验通过后该路径作为 workspace 生效
- **THEN** 校验失败时返回清晰错误原因（例如路径不存在/不可访问）
