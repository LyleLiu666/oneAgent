## ADDED Requirements

### Requirement: System MUST gate admin endpoints by admin token/principal
系统必须 (MUST) 将“安全敏感的管理操作”（例如 token 管理、tool policy 管理）限定为 admin principal 才可访问。

至少包括以下管理 API（或等价机制）：
- 列出/创建/吊销 auth token
- 读取/写入 tool permissions policy

#### Scenario: Non-admin token cannot manage tokens
- **GIVEN** 一个非 admin token（principal_id=bob）
- **WHEN** 调用 `/api/admin/tokens`（或等价）
- **THEN** 系统返回 HTTP 403（或等价），并给出可解释错误

#### Scenario: Admin token can create and revoke tokens
- **GIVEN** 一个 admin token（principal_id=local，role=admin）
- **WHEN** 通过管理 API 创建一个新 token 并指定 `principal_id=alice`
- **THEN** 系统返回新 token 的明文值（仅此一次）与元数据（principal_id/created_at）
- **WHEN** admin 吊销该 token
- **THEN** 该 token 后续访问任意受保护 API 时返回 HTTP 401（或等价）

### Requirement: System MUST provide a pairing-based token issuance flow
系统必须 (MUST) 提供一种“配对”方式为新设备/新用户发放 token，避免直接复制粘贴 admin token（例如：短时 pairing code → 换取长期 token）。

#### Scenario: Pairing code expires and cannot be reused
- **GIVEN** 系统生成一个短时 pairing code（例如 60s TTL）
- **WHEN** pairing code 过期或已被使用
- **THEN** 该 code 不可再次换取 token，并返回可理解的失败原因
