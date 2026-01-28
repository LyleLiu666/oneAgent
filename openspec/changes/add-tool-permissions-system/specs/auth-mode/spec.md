## ADDED Requirements

### Requirement: Token 模式必须支持多 principal（多 token 映射）
系统必须 (MUST) 在 `AUTH_MODE=token` 下支持多个有效 token，并将每个 token 映射到一个稳定的 `principal_id`，以支撑 per-user 的 tool permissions 与数据隔离。

系统必须 (MUST) 将解析得到的 `principal_id` 注入到后端请求上下文（例如 Gin context），并在 tool 执行上下文中可用。

#### Scenario: 不同 token 映射到不同 principal_id
- **GIVEN** 系统存在两个有效 token，分别映射到 principal `alice` 与 `bob`
- **WHEN** 客户端以 `Authorization: Bearer <alice-token>` 访问受保护 API
- **THEN** 该请求的 `principal_id=alice`
- **WHEN** 客户端以 `Authorization: Bearer <bob-token>` 访问受保护 API
- **THEN** 该请求的 `principal_id=bob`

