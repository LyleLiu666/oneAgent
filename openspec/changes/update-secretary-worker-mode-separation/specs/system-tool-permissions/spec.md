# system-tool-permissions Spec Delta

## ADDED Requirements

### Requirement: Secretary agent MUST run with a read-only filesystem policy (no workspace mutation)
系统必须 (MUST) 为秘书 agent 提供默认只读的文件系统权限策略（best-effort），以保证“秘书可查询/解释，但不亲自干活（不改用户资产）”：
- 不得 (MUST NOT) 在用户 workspace 内执行任何增/删/改文件操作（fail-closed）
- 允许 (MAY) 执行只读文件操作用于证据检索（best-effort），但不得绕过 policy
- 若某个工具的安全元信息未知，默认按 `mutating+irreversible` 处理并对秘书 fail-closed（best-effort）

#### Scenario: Secretary cannot write files even if the tool name is known
- **GIVEN** 用户处于秘书模式（best-effort）
- **WHEN** 秘书尝试调用任意写文件/删文件/改文件工具（best-effort）
- **THEN** 系统拒绝执行并返回可解释的权限错误（best-effort）
- **AND** 该错误会作为 tool_result 写回消息列表（append-only），供秘书自行选择替代路径或派工（best-effort）

#### Scenario: Secretary can read evidence files under policy
- **GIVEN** 用户处于秘书模式（best-effort）
- **WHEN** 秘书调用只读文件工具读取某个产物文件（例如 findings/trace）（best-effort）
- **THEN** 系统在权限允许的前提下返回文件内容（best-effort）
