# app-shell-ux Specification

## Purpose
TBD - created by archiving change update-app-shell-secretary-first. Update Purpose after archive.
## Requirements
### Requirement: App MUST support a global UI mode (secretary/full)
系统必须 (MUST) 提供全局 `ui_mode`，并至少包含两种值：
- `secretary`：默认低噪声，仅保留对话主路径
- `full`：高级模式，展示全量工作台与导航

系统必须 (MUST) 将 `ui_mode` 持久化（例如 localStorage），并在刷新/重启后保持一致（best-effort）。

#### Scenario: Default ui_mode is secretary on first launch
- **GIVEN** 用户首次打开应用且未存储任何 `ui_mode` 偏好
- **WHEN** 用户进入应用主界面
- **THEN** 系统以 `ui_mode=secretary` 呈现（best-effort）

#### Scenario: ui_mode persists across reload
- **GIVEN** 用户已将 `ui_mode` 切换为 `full`
- **WHEN** 用户刷新页面或重启应用
- **THEN** 系统仍以 `ui_mode=full` 呈现（best-effort）

### Requirement: Sidebar MUST NOT render in secretary mode
系统必须 (MUST) 在 `ui_mode=secretary` 时隐藏全局 Sidebar（含移动端菜单按钮/抽屉入口），以避免“管理系统”外观污染默认体验。

#### Scenario: Sidebar is hidden in secretary mode
- **GIVEN** 用户已登录且 `ui_mode=secretary`
- **WHEN** 用户打开任意页面（Chat/Settings/Tasks 等，best-effort）
- **THEN** 页面不渲染 Sidebar（best-effort）
- **AND** 用户无法通过 UI 打开 Sidebar（best-effort）

### Requirement: Sidebar MUST render in full mode
系统必须 (MUST) 在 `ui_mode=full` 时展示 Sidebar，并提供到全量页面的导航入口（best-effort）。

#### Scenario: Sidebar is visible in full mode
- **GIVEN** 用户已登录且 `ui_mode=full`
- **WHEN** 用户打开任意页面
- **THEN** 页面展示 Sidebar（best-effort）

### Requirement: Deep links MUST provide an escape hatch in secretary mode (best-effort)
当用户在 `ui_mode=secretary` 下通过 URL 进入一个“完全模式页面”（例如 Tasks/Governance/Ledger），系统必须 (MUST) 通过低噪声方式提示其“该页属于完全模式”，并提供一键切换到 `ui_mode=full` 的入口（best-effort）。

#### Scenario: User can switch to full mode from a deep-linked full page
- **GIVEN** 用户在 `ui_mode=secretary` 下访问一个完全模式页面（best-effort）
- **WHEN** 系统提示并提供“进入完全模式”按钮（best-effort）
- **AND** 用户点击该按钮
- **THEN** 系统切换到 `ui_mode=full` 并显示 Sidebar（best-effort）

