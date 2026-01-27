# llm-provider-management Specification

## Purpose
TBD - created by archiving change update-workspace-first-onboarding. Update Purpose after archive.
## Requirements
### Requirement: 管理 LLM Providers（多平台接入）
系统必须 (MUST) 支持管理多个 LLM Provider（多平台接入），至少包括：创建、列出、更新、删除 Provider，并安全存储 Provider 的敏感凭证（例如 API Key）。

#### Scenario: 创建并列出 Provider
- **GIVEN** 用户已登录
- **WHEN** 用户创建一个 Provider（包含 `provider_type`、`base_url` 与 `api_key`）
- **THEN** 用户可在 Provider 列表中看到该 Provider
- **THEN** 列表响应不得泄露明文 `api_key`（仅允许返回 `has_api_key=true/false` 或等价字段）

### Requirement: 管理 Models（按 Provider 归属）
系统必须 (MUST) 支持为指定 Provider 管理多个模型配置（Models），至少包括：创建、列出、更新、删除，以及设置默认模型（default）。

#### Scenario: 为 Provider 创建模型并设为默认
- **GIVEN** 系统中存在一个 Provider
- **WHEN** 用户为该 Provider 创建一个 Model，并设置 `is_default=true`
- **THEN** 该 Provider 下的默认模型为新创建的 Model
- **THEN** 同一 Provider 下默认模型最多为 1 个（新默认会覆盖旧默认）

### Requirement: 会话级模型选择（用于聊天）
系统必须 (MUST) 支持在会话级别选择本次聊天使用的模型，并将选择结果持久化为会话 metadata，以便刷新页面或重载会话时保持一致。

#### Scenario: UI 切换模型后写入会话 metadata
- **GIVEN** 用户打开一个聊天会话
- **WHEN** 用户在 UI 中切换当前模型
- **THEN** 后续发起的聊天请求使用新模型
- **THEN** 该会话在重新加载后仍显示该模型为当前模型（来自会话 metadata）

