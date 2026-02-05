# app-shell-ux Spec Delta

## MODIFIED Requirements

### Requirement: App MUST support a global UI mode (secretary/full)
系统必须 (MUST) 提供全局 `ui_mode`，并至少包含两种值：
- `secretary`：默认低噪声，并以“与秘书对话”为默认入口（best-effort）
- `full`：完整工作台，并以“与 worker 对话”为默认入口（best-effort）

系统必须 (MUST) 将 `ui_mode` 持久化（例如 localStorage），并在刷新/重启后保持一致（best-effort）。

#### Scenario: Default entry respects ui_mode on first launch
- **GIVEN** 用户首次打开应用且未存储任何 `ui_mode` 偏好
- **WHEN** 用户进入应用主界面
- **THEN** 系统以 `ui_mode=secretary` 呈现（best-effort）
- **AND** 系统默认进入 `/secretary`（best-effort）

#### Scenario: Switching ui_mode switches route and preserves conversations
- **GIVEN** 用户在 `ui_mode=secretary` 下停留在 `/secretary` 且已有 secretary 会话内容（best-effort）
- **WHEN** 用户切换到 `ui_mode=full`
- **THEN** 系统跳转到 `/chat`（best-effort）
- **AND** 系统展示 worker chat 会话（best-effort）
- **WHEN** 用户切换回 `ui_mode=secretary`
- **THEN** 系统跳回 `/secretary` 并恢复之前的 secretary 会话（best-effort）

