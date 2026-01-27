## ADDED Requirements

### Requirement: Task queue 默认 limits（steps/runtime）
系统必须 (MUST) 为 task queue 提供默认 limits（最大步骤数、最大运行时长），用于避免“无限运行/无限循环”导致资源失控。

系统应该 (SHOULD) 允许通过环境变量覆盖默认 limits 与上限（cap），以便不同团队按安全/成本约束治理。

#### Scenario: 创建 task 未指定 limits 时使用默认值
- **WHEN** 用户创建 task 且未提供 `limits.max_steps` 与 `limits.max_runtime_seconds`
- **THEN** 系统返回的 task 必须包含默认 limits
- **AND** runner 执行该 task 时必须使用同样的默认 limits

#### Scenario: 用户请求过大的 limits 会被 cap 限制
- **GIVEN** 系统配置了 limits cap
- **WHEN** 用户创建 task 时请求的 limits 超过 cap
- **THEN** 系统应将 limits 限制在 cap 范围内（并在返回的 task 中反映）

