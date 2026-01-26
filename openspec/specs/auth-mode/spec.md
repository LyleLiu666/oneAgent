# auth-mode Specification

## Purpose
TBD - created by archiving change refactor-container-to-local-tool. Update Purpose after archive.
## Requirements
### Requirement: 默认生成本地访问令牌（不过期）
系统必须 (MUST) 支持 `AUTH_MODE=token` 作为默认认证模式，并在未显式配置 token 值时于启动时自动生成一个随机 token（不过期）。

该 token 只是一个本地访问凭证（opaque string），系统不得 (MUST NOT) 要求其满足 JWT 结构或包含过期声明。

系统必须 (MUST) 将 token 以固定文件路径持久化到 `ONEAGENT_HOME/.oneagent/config/auth_token`（若文件不存在则生成并写入；若存在则复用）。

为兼容历史配置，系统应该 (SHOULD) 继续接受 `AUTH_MODE=password` 作为 `token` 的别名（行为一致）。

#### Scenario: local/dev 默认启用 token
- **WHEN** 用户以 `local` profile 启动且未显式设置 `AUTH_MODE`
- **THEN** 系统默认使用 `AUTH_MODE=token`
- **WHEN** 用户以 `dev` profile 启动且未显式设置 `AUTH_MODE`
- **THEN** 系统默认使用 `AUTH_MODE=token`

#### Scenario: token 文件缺失时自动生成
- **GIVEN** `ONEAGENT_HOME/.oneagent/config/auth_token` 不存在
- **WHEN** 服务启动
- **THEN** 系统生成 token 并写入该文件

#### Scenario: doctor 提示 token 文件路径
- **WHEN** 用户执行 `oneagent doctor`
- **THEN** 输出包含 `ONEAGENT_HOME/.oneagent/config/auth_token` 的路径提示（而不是明文 token）

### Requirement: Token 模式下 API 必须携带访问令牌
系统必须 (MUST) 在 `AUTH_MODE=token` 时要求所有受保护 API 请求携带访问令牌；token 错误或缺失时应拒绝请求。

#### Scenario: 缺失或错误 token 被拒绝
- **WHEN** `AUTH_MODE=token` 且客户端未携带 token 访问任意受保护 API
- **THEN** 系统返回 HTTP 401（或等价的未授权响应）
- **WHEN** `AUTH_MODE=token` 且客户端携带错误 token 访问任意受保护 API
- **THEN** 系统返回 HTTP 401（或等价的未授权响应）

#### Scenario: 通过 Authorization 头携带 token
- **WHEN** `AUTH_MODE=token` 且客户端以 `Authorization: Bearer <token>` 形式携带 token 访问受保护 API
- **THEN** 系统允许该请求通过鉴权

### Requirement: 登录页面提示局域网风险
系统必须 (MUST) 在登录页面（或等价入口）明确提示：该产品仅建议在可信局域网内使用；将服务暴露到公网有非常大风险。

#### Scenario: 登录页包含风险提示
- **WHEN** 用户打开登录页面
- **THEN** 页面包含“仅建议在局域网内使用/公网风险很大”的明确提示文本

### Requirement: 支持显式关闭认证（仅限开发/离线极简）
系统必须 (MUST) 支持 `AUTH_MODE=none`，用于开发或离线极简场景；该模式必须显式开启，且启动日志中包含明确风险提示。

#### Scenario: none 模式需要显式开启
- **WHEN** 用户未显式设置 `AUTH_MODE=none`
- **THEN** 系统不得在无认证模式下启动（默认仍为 token）
- **WHEN** 用户显式设置 `AUTH_MODE=none` 并启动
- **THEN** 系统启动日志中包含“无认证风险提示”

### Requirement: 废弃 OAuth/Keycloak 登录链路
系统必须 (MUST) 在本地工具形应用的默认路径中不再依赖 OAuth/Keycloak 登录链路，并将相关接口标记为废弃（可返回明确的废弃错误）。

#### Scenario: OAuth 相关接口返回废弃提示
- **WHEN** 用户在本地工具模式下请求 `/api/auth/config` 或 `/api/auth/callback`
- **THEN** 系统返回明确的废弃响应（例如 HTTP 410 或 404，并包含可理解的错误信息）

