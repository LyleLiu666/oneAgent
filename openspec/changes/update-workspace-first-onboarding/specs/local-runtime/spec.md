## ADDED Requirements
### Requirement: Quick-start serve flags（`--open` / `--workspace`）
系统必须 (MUST) 为 `oneagent serve` 提供 quick-start flags，以降低“安装后第一次使用”的操作成本：
- `--open`：服务启动成功后，自动打开默认浏览器访问 UI。
- `--workspace <path>`：设置一个默认 workspace，供 UI 在首次进入/新会话时自动填充（用户仍可在会话级覆盖）。

#### Scenario: `--open` 自动打开 UI
- **WHEN** 用户执行 `oneagent serve --open`
- **THEN** 服务启动成功后，系统尝试打开默认浏览器访问 UI
- **THEN** 若打开失败，系统输出可操作的提示但服务仍保持运行

#### Scenario: `--workspace` 提供默认 workspace
- **WHEN** 用户执行 `oneagent serve --workspace /path/to/ws`
- **THEN** 服务向 UI 暴露 default workspace = `/path/to/ws`
- **THEN** UI 在未显式设置会话 workspace 的情况下，默认填充该 workspace

