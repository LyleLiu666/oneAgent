## ADDED Requirements

### Requirement: System MUST support a loopback-only secure bind mode
系统必须 (MUST) 支持将服务仅绑定在 loopback 地址（例如 `127.0.0.1` 或 `::1`），用于“同机访问最小暴露”模式。

#### Scenario: Serve binds loopback successfully
- **WHEN** 用户执行 `oneagent serve --bind 127.0.0.1`
- **THEN** 服务启动成功且 UI 可通过 `http://localhost:<port>` 访问

#### Scenario: Non-loopback bind shows strong warning
- **WHEN** 用户执行 `oneagent serve --bind 0.0.0.0`（或 `--bind ::`）
- **THEN** 系统在启动日志与 UI 中明确提示“局域网/公网暴露风险很大”
- **AND** 给出安全替代方案（loopback/Tailscale）

### Requirement: Doctor MUST report network exposure posture
系统必须 (MUST) 在 `oneagent doctor`（或等价诊断）中报告当前 bind 地址是否为 loopback，并在非 loopback 时给出可操作的安全建议（例如使用 loopback + Tailscale）。

#### Scenario: doctor shows actionable suggestion when exposed
- **GIVEN** 当前配置 bind 为非 loopback
- **WHEN** 用户执行 `oneagent doctor`
- **THEN** doctor 输出包含风险提示与替代建议（可操作命令或指引）
