## ADDED Requirements
### Requirement: Windows 下可运行（Local Tool）
系统必须 (MUST) 支持在 Windows 下以本地工具形应用运行：`oneagent serve` 与 `oneagent doctor` 必须可用。

#### Scenario: Windows 上 serve/doctor 可用
- **GIVEN** 用户在 Windows 下安装了 oneAgent 可执行文件
- **WHEN** 用户执行 `oneagent serve`
- **THEN** 服务启动成功（UI + API）
- **WHEN** 用户执行 `oneagent doctor`
- **THEN** doctor 输出包含 Windows 平台的诊断信息与依赖可用性

