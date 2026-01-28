## MODIFIED Requirements

### Requirement: 默认工具权限与文件作用域 (Default Tools & File Scope)
系统必须 (MUST) 默认允许子 Agent 使用与主 Agent 相同的工具集合，以支持生产级交付；但子 Agent 的 effective tools 必须 (MUST) 以父上下文的 tool permissions 作为上限（见 `system-tool-permissions`），不得 (MUST NOT) 获得比父上下文更高的权限。

系统必须 (MUST) 支持对子 Agent 的“可写文件范围”进行限制（scope，glob 规则，基于 `<workspace>/` 的相对路径），并在文件工具层强制执行：超出 scope 的写/改/删请求必须被拒绝并返回可理解错误。

#### Scenario: 子 Agent 请求父上下文不允许的工具会被拒绝
- **GIVEN** 父上下文的 tool permissions 拒绝 `bash`
- **WHEN** 子 Agent 在 `tool_ids` 中请求包含 `bash`
- **THEN** 系统拒绝挂载该 tool，并返回清晰错误（例如 “tool is denied by parent policy”）

#### Scenario: 子 Agent 越界修改文件被拒绝
- **GIVEN** 当前会话启用 workspace，根目录为 `<workspace>/`，并且子 Agent scope 被限制为 `backend/**`
- **WHEN** 子 Agent 尝试修改 `frontend/App.vue`
- **THEN** 系统拒绝该写/改/删操作，并返回清晰错误（例如 “path is outside subagent scope”）

