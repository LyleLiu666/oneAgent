# 规范: 认证模式 (Auth Modes)

## ADDED Requirements

### Requirement: 默认使用共享密码作为访问控制
系统必须 (MUST) 支持 `AUTH_MODE=password`，并在未显式配置时默认启用该模式（以满足“局域网可访问但不裸奔”的默认安全要求）。

#### Scenario: local/dev 默认启用 password
- **WHEN** 用户以 `local` profile 启动且未显式设置 `AUTH_MODE`
- **THEN** 系统默认使用 `AUTH_MODE=password`
- **WHEN** 用户以 `dev` profile 启动且未显式设置 `AUTH_MODE`
- **THEN** 系统默认使用 `AUTH_MODE=password`

### Requirement: Password 模式下 API 必须携带共享密码
系统必须 (MUST) 在 `AUTH_MODE=password` 时要求所有受保护 API 请求携带共享密码；密码错误或缺失时应拒绝请求。

#### Scenario: 缺失或错误密码被拒绝
- **WHEN** `AUTH_MODE=password` 且客户端未携带密码访问任意受保护 API
- **THEN** 系统返回 HTTP 401（或等价的未授权响应）
- **WHEN** `AUTH_MODE=password` 且客户端携带错误密码访问任意受保护 API
- **THEN** 系统返回 HTTP 401（或等价的未授权响应）

#### Scenario: 通过 Authorization 头携带共享密码
- **WHEN** `AUTH_MODE=password` 且客户端以 `Authorization: Bearer <password>` 形式携带共享密码访问受保护 API
- **THEN** 系统允许该请求通过鉴权

### Requirement: 支持显式关闭认证（仅限开发/离线极简）
系统必须 (MUST) 支持 `AUTH_MODE=none`，用于开发或离线极简场景；该模式必须显式开启，且启动日志中包含明确风险提示。

#### Scenario: none 模式需要显式开启
- **WHEN** 用户未显式设置 `AUTH_MODE=none`
- **THEN** 系统不得在无认证模式下启动（默认仍为 password）
- **WHEN** 用户显式设置 `AUTH_MODE=none` 并启动
- **THEN** 系统启动日志中包含“无认证风险提示”

### Requirement: 废弃 OAuth/Keycloak 登录链路
系统必须 (MUST) 在本地工具形应用的默认路径中不再依赖 OAuth/Keycloak 登录链路，并将相关接口标记为废弃（可返回明确的废弃错误）。

#### Scenario: OAuth 相关接口返回废弃提示
- **WHEN** 用户在本地工具模式下请求 `/api/auth/config` 或 `/api/auth/callback`
- **THEN** 系统返回明确的废弃响应（例如 HTTP 410 或 404，并包含可理解的错误信息）

