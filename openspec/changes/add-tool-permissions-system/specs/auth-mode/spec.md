## ADDED Requirements

### Requirement: Token auth MUST support multiple principals
系统必须 (MUST) 支持多个访问 token，并将每个 token 映射到一个 `principal_id`（用于权限策略与审计）。

#### Scenario: Different tokens map to different principals
- **GIVEN** 系统存在 token A → principal `alice`，token B → principal `bob`
- **WHEN** 客户端使用 token A 访问受保护 API
- **THEN** 请求上下文中的 `principal_id=alice`
- **WHEN** 客户端使用 token B 访问受保护 API
- **THEN** 请求上下文中的 `principal_id=bob`

